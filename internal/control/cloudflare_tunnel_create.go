package control

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

var cloudflareTunnelNamePattern = regexp.MustCompile(`[^a-zA-Z0-9-]+`)

func cloudflareTunnelName(deviceID int64, siteID string) string {
	slug := strings.Trim(cloudflareTunnelNamePattern.ReplaceAllString(siteID, "-"), "-")
	if slug == "" {
		slug = "site"
	}
	if len(slug) > 32 {
		slug = slug[:32]
	}
	return fmt.Sprintf("yevqo-%s-%d", slug, deviceID)
}

func (s *Server) handleCloudflareZones(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &in); err != nil {
		apiError(w, shared.ErrBadRequest("invalid Cloudflare token request"))
		return
	}
	in.Token = strings.TrimSpace(in.Token)
	if len(in.Token) < 20 || len(in.Token) > 4096 || strings.ContainsAny(in.Token, "\r\n\x00") {
		apiError(w, shared.ErrBadRequest("enter a valid scoped Cloudflare API token"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	var verify struct {
		Status string `json:"status"`
	}
	if err := cloudflareAPIRequest(ctx, in.Token, http.MethodGet, "/user/tokens/verify", nil, &verify); err != nil || verify.Status != "active" {
		apiError(w, shared.ErrBadRequest("Cloudflare token is invalid or inactive"))
		return
	}
	type zoneItem struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
		Account struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"account"`
	}
	zones := make([]map[string]string, 0)
	for page := 1; page <= 10; page++ {
		var batch []zoneItem
		endpoint := "/zones?per_page=50&page=" + strconv.Itoa(page)
		if err := cloudflareAPIRequest(ctx, in.Token, http.MethodGet, endpoint, nil, &batch); err != nil {
			apiError(w, shared.ErrBadRequest("could not list Cloudflare zones; check Zone Read permission"))
			return
		}
		for _, zone := range batch {
			if zone.Status != "active" || !validCloudflareZoneID(zone.ID) || zone.Name == "" || !validCloudflareAccountID(zone.Account.ID) {
				continue
			}
			zones = append(zones, map[string]string{
				"id": zone.ID, "name": zone.Name,
				"account_id": zone.Account.ID, "account_name": zone.Account.Name,
			})
		}
		if len(batch) < 50 {
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "zones": zones})
}

func (s *Server) handleCloudflareTunnelCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	u := UserFromContext(r.Context())
	var in struct {
		DeviceID int64  `json:"device_id"`
		SiteID   string `json:"site_id"`
		Domain   string `json:"domain"`
	}
	if err := decodeJSON(r, &in); err != nil || in.DeviceID < 1 || !validBackupSiteID(in.SiteID) {
		apiError(w, shared.ErrBadRequest("invalid Cloudflare Tunnel request"))
		return
	}
	hostname := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(in.Domain), "."))
	if !domainPattern.MatchString(hostname) {
		apiError(w, shared.ErrBadRequest("enter a valid application hostname"))
		return
	}
	dev, err := s.db.GetDeviceByID(r.Context(), in.DeviceID)
	if err != nil {
		apiError(w, shared.ErrNotFound("device not found"))
		return
	}
	if err := s.authorizeSiteProjects(r.Context(), in.DeviceID, in.SiteID, dev.UserID, u.ID, "site.configure"); err != nil {
		apiError(w, err)
		return
	}
	if u.ID != dev.UserID {
		apiError(w, shared.ErrForbidden("only the workspace owner can manage Cloudflare tunnels"))
		return
	}
	var port int
	var currentDomain sql.NullString
	var currentTunnelToken []byte
	if err := s.db.conn.QueryRowContext(r.Context(), `SELECT domain,port,tunnel_token_enc FROM sites WHERE device_id=? AND site_id=?`, in.DeviceID, in.SiteID).Scan(&currentDomain, &port, &currentTunnelToken); err != nil {
		apiError(w, shared.ErrNotFound("site not found"))
		return
	}
	if port < 1024 || port > 65535 {
		apiError(w, shared.ErrBadRequest("application port is outside the supported range"))
		return
	}
	if len(currentTunnelToken) > 0 {
		apiError(w, shared.ErrConflict("this application already has a Tunnel; remove its current domain connection first"))
		return
	}
	var existingRoutes int
	if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM site_cloudflare_routes WHERE device_id=? AND site_id=?`, in.DeviceID, in.SiteID).Scan(&existingRoutes); err != nil {
		apiError(w, shared.ErrInternal("could not check existing Cloudflare route"))
		return
	}
	if existingRoutes > 0 {
		apiError(w, shared.ErrConflict("remove the existing Cloudflare route before creating another Tunnel"))
		return
	}
	accountID, zoneID, zoneName, apiToken, err := s.cloudflareCredentials(r.Context(), dev.UserID)
	if err != nil {
		apiError(w, shared.ErrBadRequest("connect a Cloudflare API token and zone first"))
		return
	}
	if !hostnameBelongsToZone(hostname, zoneName) {
		apiError(w, shared.ErrBadRequest("application hostname must belong to the selected Cloudflare zone"))
		return
	}
	var occupied int
	if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM site_cloudflare_routes WHERE user_id=? AND hostname=?`, dev.UserID, hostname).Scan(&occupied); err != nil {
		apiError(w, shared.ErrInternal("could not check hostname assignment"))
		return
	}
	if occupied > 0 {
		apiError(w, shared.ErrConflict("this hostname is already assigned to another application"))
		return
	}
	conn, online := s.hub.Get(in.DeviceID)
	if !online {
		apiError(w, shared.ErrBadRequest("connect the Android Agent before creating a Tunnel"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Minute+30*time.Second)
	defer cancel()

	installCtx, installCancel := context.WithTimeout(ctx, 5*time.Minute+15*time.Second)
	installed, installErr := conn.SendCommand(installCtx, "install_cloudflared", map[string]interface{}{})
	installCancel()
	if installErr != nil || installed == nil || !installed.Ok {
		message := "Agent could not install cloudflared"
		if installed != nil && installed.Message != "" {
			message = installed.Message
		}
		apiError(w, shared.ErrBadRequest(message))
		return
	}

	var created struct {
		ID         string `json:"id"`
		AccountTag string `json:"account_tag"`
		ConfigSrc  string `json:"config_src"`
		Token      string `json:"token"`
	}
	createBody := map[string]interface{}{"name": cloudflareTunnelName(in.DeviceID, in.SiteID), "config_src": "cloudflare"}
	if err := cloudflareAPIRequest(ctx, apiToken, http.MethodPost, "/accounts/"+accountID+"/cfd_tunnel", createBody, &created); err != nil {
		apiError(w, shared.ErrBadRequest("Cloudflare could not create the Tunnel; check Cloudflare Tunnel Edit permission"))
		return
	}
	if !validCloudflareTunnelID(created.ID) || created.AccountTag != accountID || (created.ConfigSrc != "" && created.ConfigSrc != "cloudflare") {
		apiError(w, shared.ErrInternal("Cloudflare returned an invalid Tunnel identity"))
		return
	}
	tunnelID := created.ID
	cleanupTunnel := func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		_ = updateCloudflareTunnelIngress(cleanupCtx, apiToken, accountID, tunnelID, hostname, "", true, false)
		_ = deleteCloudflareTunnelDNS(cleanupCtx, apiToken, zoneID, hostname, tunnelID)
		_ = cloudflareAPIRequest(cleanupCtx, apiToken, http.MethodDelete, "/accounts/"+accountID+"/cfd_tunnel/"+tunnelID, nil, nil)
	}
	tunnelToken := strings.TrimSpace(created.Token)
	if tunnelToken == "" {
		if err := cloudflareAPIRequest(ctx, apiToken, http.MethodGet, "/accounts/"+accountID+"/cfd_tunnel/"+tunnelID+"/token", nil, &tunnelToken); err != nil {
			cleanupTunnel()
			apiError(w, shared.ErrBadRequest("Tunnel was created but Cloudflare did not return its connector token"))
			return
		}
	}
	if len(tunnelToken) < 20 || len(tunnelToken) > 4096 || strings.ContainsAny(tunnelToken, "\r\n\x00") {
		cleanupTunnel()
		apiError(w, shared.ErrInternal("Cloudflare returned an invalid Tunnel token"))
		return
	}
	service := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := updateCloudflareTunnelIngress(ctx, apiToken, accountID, tunnelID, hostname, service, false, false); err != nil {
		cleanupTunnel()
		apiError(w, shared.ErrBadRequest("could not configure Tunnel ingress: "+err.Error()))
		return
	}
	record, err := ensureCloudflareTunnelDNS(ctx, apiToken, zoneID, hostname, tunnelID)
	if err != nil {
		cleanupTunnel()
		apiError(w, shared.ErrBadRequest("could not create the proxied DNS record: "+err.Error()))
		return
	}
	nonce, ciphertext, err := encryptCredential(s.credentialKey, []byte(tunnelToken))
	if err != nil {
		cleanupTunnel()
		apiError(w, shared.ErrInternal("could not encrypt Tunnel token"))
		return
	}
	agentCtx, agentCancel := context.WithTimeout(ctx, 60*time.Second)
	result, agentErr := conn.SendCommand(agentCtx, "create_tunnel", &shared.CreateTunnel{
		SiteId: in.SiteID, Hostname: hostname, TunnelToken: tunnelToken, Port: uint32(port),
	})
	agentCancel()
	if agentErr != nil || result == nil || !result.Ok {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
		_, _ = conn.SendCommand(stopCtx, "stop_tunnel", &shared.StopTunnel{SiteId: in.SiteID})
		stopCancel()
		cleanupTunnel()
		message := "Agent could not start cloudflared"
		if result != nil && result.Message != "" {
			message = result.Message
		}
		apiError(w, shared.ErrBadRequest(message))
		return
	}
	tx, err := s.db.conn.BeginTx(ctx, nil)
	if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE sites SET domain=?,tunnel_token_enc=?,tunnel_status='active',tunnel_error=NULL,tunnel_updated_at=NOW() WHERE device_id=? AND site_id=?`, hostname, append(nonce, ciphertext...), in.DeviceID, in.SiteID)
	}
	if err == nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO site_cloudflare_routes(user_id,device_id,site_id,tunnel_id,hostname,origin_port,ingress_managed) VALUES(?,?,?,?,?,?,1)`, dev.UserID, in.DeviceID, in.SiteID, tunnelID, hostname, port)
	}
	if err == nil {
		err = tx.Commit()
	} else if tx != nil {
		_ = tx.Rollback()
	}
	if err != nil {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
		_, _ = conn.SendCommand(stopCtx, "stop_tunnel", &shared.StopTunnel{SiteId: in.SiteID})
		stopCancel()
		cleanupTunnel()
		apiError(w, shared.ErrInternal("Tunnel started but Yevqo could not save its route; cleanup was attempted"))
		return
	}
	_ = s.db.Log(r.Context(), &shared.LogEntry{
		UserID: shared.PtrInt64(u.ID), DeviceID: shared.PtrInt64(in.DeviceID),
		Action: "cloudflare_tunnel_created",
		Details: map[string]interface{}{"site_id": in.SiteID, "hostname": hostname, "tunnel_id": tunnelID, "origin_port": port},
		IP: clientIP(r),
	})
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"ok": true, "configured": true, "tunnel_id": tunnelID, "hostname": hostname,
		"origin": service, "dns_record": record, "ingress_managed": true, "tunnel_status": "active",
	})
}
