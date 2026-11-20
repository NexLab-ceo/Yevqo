package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

const cloudflareAPIBase = "https://api.cloudflare.com/client/v4"
var cloudflareZoneIDPattern = regexp.MustCompile("^[a-fA-F0-9]{32}$")
var cloudflareDNSRecordIDPattern = regexp.MustCompile("^[a-fA-F0-9]{32}$")
var cloudflareTunnelIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type cloudflareAPIEnvelope struct {
	Success bool `json:"success"`
	Result json.RawMessage `json:"result"`
	Errors []struct { Message string `json:"message"` } `json:"errors"`
}

func cloudflareAPIRequest(ctx context.Context, token, method, endpoint string, body interface{}, target interface{}) error {
	var reader io.Reader
	if body != nil { encoded,err:=json.Marshal(body);if err!=nil{return err};reader=strings.NewReader(string(encoded)) }
	req,err:=http.NewRequestWithContext(ctx,method,cloudflareAPIBase+endpoint,reader);if err!=nil{return err}
	req.Header.Set("Authorization","Bearer "+token)
	req.Header.Set("Content-Type","application/json")
	req.Header.Set("Accept","application/json")
	resp,err:=integrationHTTPClient.Do(req);if err!=nil{return err};defer resp.Body.Close()
	var envelope cloudflareAPIEnvelope
	if err:=json.NewDecoder(io.LimitReader(resp.Body,2<<20)).Decode(&envelope);err!=nil{return err}
	if resp.StatusCode<200||resp.StatusCode>=300||!envelope.Success {
		if len(envelope.Errors)>0{return fmt.Errorf("Cloudflare API: %s",envelope.Errors[0].Message)}
		return fmt.Errorf("Cloudflare API returned HTTP %d",resp.StatusCode)
	}
	if target!=nil && len(envelope.Result)>0 { if err:=json.Unmarshal(envelope.Result,target);err!=nil{return err} }
	return nil
}

func validCloudflareZoneID(value string) bool { return cloudflareZoneIDPattern.MatchString(value) }
func validCloudflareTunnelID(value string) bool { return cloudflareTunnelIDPattern.MatchString(value) }
func validCloudflareDNSRecordID(value string) bool { return cloudflareDNSRecordIDPattern.MatchString(value) }
func hostnameBelongsToZone(hostname, zone string) bool {
	hostname=strings.ToLower(strings.TrimSuffix(strings.TrimSpace(hostname),"."))
	zone=strings.ToLower(strings.TrimSuffix(strings.TrimSpace(zone),"."))
	return hostname==zone || strings.HasSuffix(hostname,"."+zone)
}

func validCloudflareAccountID(value string) bool { return cloudflareZoneIDPattern.MatchString(value) }

func (s *Server) cloudflareCredentials(ctx context.Context,userID int64)(string,string,string,string,error){
	var accountID,zoneID,zoneName string;var nonce,ciphertext []byte
	err:=s.db.conn.QueryRowContext(ctx,`SELECT account_id,zone_id,zone_name,token_nonce,token_ciphertext FROM cloudflare_accounts WHERE user_id=?`,userID).Scan(&accountID,&zoneID,&zoneName,&nonce,&ciphertext)
	if err!=nil{return "","","","",err}
	token,err:=decryptCredential(s.credentialKey,nonce,ciphertext)
	if err!=nil{return "","","","",err}
	if accountID=="" {
		var zone struct{Account struct{ID string `json:"id"`} `json:"account"`}
		if err:=cloudflareAPIRequest(ctx,string(token),http.MethodGet,"/zones/"+zoneID,nil,&zone);err!=nil||!validCloudflareAccountID(zone.Account.ID){return "","","","",fmt.Errorf("could not resolve Cloudflare account ID; reconnect the zone")}
		accountID=zone.Account.ID
		if _,err:=s.db.conn.ExecContext(ctx,`UPDATE cloudflare_accounts SET account_id=? WHERE user_id=?`,accountID,userID);err!=nil{return "","","","",err}
	}
	return accountID,zoneID,zoneName,string(token),nil
}

func (s *Server) handleCloudflareConfig(w http.ResponseWriter,r *http.Request){
	u:=UserFromContext(r.Context())
	switch r.Method {
	case http.MethodGet:
		var accountID,zoneID,zoneName string
		err:=s.db.conn.QueryRowContext(r.Context(),`SELECT account_id,zone_id,zone_name FROM cloudflare_accounts WHERE user_id=?`,u.ID).Scan(&accountID,&zoneID,&zoneName)
		if err==sql.ErrNoRows{writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"connected":false});return};if err!=nil{apiError(w,shared.ErrInternal("could not load Cloudflare configuration"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"connected":true,"account_id":accountID,"zone_id":zoneID,"zone_name":zoneName})
	case http.MethodPost:
		var in struct{Token string `json:"token"`;ZoneID string `json:"zone_id"`}
		if err:=decodeJSON(r,&in);err!=nil{apiError(w,shared.ErrBadRequest("invalid Cloudflare configuration"));return}
		in.Token=strings.TrimSpace(in.Token);in.ZoneID=strings.TrimSpace(in.ZoneID)
		if len(in.Token)<20||len(in.Token)>4096||strings.ContainsAny(in.Token,"\r\n\x00")||!validCloudflareZoneID(in.ZoneID){apiError(w,shared.ErrBadRequest("enter a valid scoped API token and zone ID"));return}
		ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second);defer cancel()
		var verify struct{Status string `json:"status"`}
		if err:=cloudflareAPIRequest(ctx,in.Token,http.MethodGet,"/user/tokens/verify",nil,&verify);err!=nil||verify.Status!="active"{apiError(w,shared.ErrBadRequest("Cloudflare token is invalid or inactive"));return}
		var zone struct{ID string `json:"id"`;Name string `json:"name"`;Status string `json:"status"`;Account struct{ID string `json:"id"`} `json:"account"`}
		if err:=cloudflareAPIRequest(ctx,in.Token,http.MethodGet,"/zones/"+in.ZoneID,nil,&zone);err!=nil||zone.ID!=in.ZoneID||zone.Name==""||zone.Status!="active"||!validCloudflareAccountID(zone.Account.ID){apiError(w,shared.ErrBadRequest("Cloudflare zone or account could not be verified"));return}
		nonce,ciphertext,err:=encryptCredential(s.credentialKey,[]byte(in.Token));if err!=nil{apiError(w,shared.ErrInternal("could not encrypt Cloudflare token"));return}
		_,err=s.db.conn.ExecContext(r.Context(),`INSERT INTO cloudflare_accounts(user_id,account_id,zone_id,zone_name,token_nonce,token_ciphertext,last_verified_at) VALUES(?,?,?,?,?,?,UTC_TIMESTAMP()) ON DUPLICATE KEY UPDATE account_id=VALUES(account_id),zone_id=VALUES(zone_id),zone_name=VALUES(zone_name),token_nonce=VALUES(token_nonce),token_ciphertext=VALUES(token_ciphertext),last_verified_at=VALUES(last_verified_at)`,u.ID,zone.Account.ID,in.ZoneID,zone.Name,nonce,ciphertext)
		if err!=nil{apiError(w,shared.ErrInternal("could not save Cloudflare configuration"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"connected":true,"account_id":zone.Account.ID,"zone_id":zone.ID,"zone_name":zone.Name})
	case http.MethodDelete:
		var routes int
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM site_cloudflare_routes WHERE user_id=?`,u.ID).Scan(&routes);err!=nil{apiError(w,shared.ErrInternal("could not check managed Cloudflare routes"));return}
		if routes>0{apiError(w,shared.ErrConflict("remove all managed site routes before disconnecting Cloudflare"));return}
		_,err:=s.db.conn.ExecContext(r.Context(),`DELETE FROM cloudflare_accounts WHERE user_id=?`,u.ID)
		if err!=nil{apiError(w,shared.ErrInternal("could not disconnect Cloudflare"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"connected":false})
	default:http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}

type cloudflareDNSRecord struct {
	ID string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
	Content string `json:"content"`
	Proxied bool `json:"proxied"`
	TTL int `json:"ttl"`
}


type cloudflareTunnelConfig struct {
	Source string `json:"source"`
	Config struct { Ingress []map[string]json.RawMessage `json:"ingress"`; OriginRequest json.RawMessage `json:"originRequest"` } `json:"config"`
}

func updateCloudflareIngressRules(rules []map[string]json.RawMessage, hostname, service string, remove, allowReplace bool) ([]map[string]json.RawMessage, bool, error) {
	isCatchall := func(rule map[string]json.RawMessage) bool {
		_, hasHostname := rule["hostname"]
		_, hasPath := rule["path"]
		return !hasHostname && !hasPath
	}
	matches := make([]int,0,1)
	for i, rule := range rules {
		if raw,ok:=rule["hostname"];ok { var current string; if err:=json.Unmarshal(raw,&current);err!=nil{return nil,false,fmt.Errorf("invalid existing ingress hostname")};if strings.EqualFold(current,hostname){matches=append(matches,i)} }
	}
	if len(matches)>1 { return nil,false,fmt.Errorf("multiple ingress rules already use this hostname") }
	changed:=false
	if remove {
		if len(matches)==1 { rules=append(rules[:matches[0]],rules[matches[0]+1:]...);changed=true }
	} else {
		serviceJSON,_:=json.Marshal(service)
		if len(matches)==1 {
			currentService:=string(rules[matches[0]]["service"])
			if currentService!=string(serviceJSON) && !allowReplace { return nil,false,fmt.Errorf("an existing ingress rule for this hostname points to another origin; review or remove it manually first") }
			if currentService!=string(serviceJSON) { rules[matches[0]]["service"]=serviceJSON;changed=true }
		} else {
			hostJSON,_:=json.Marshal(hostname)
			rule:=map[string]json.RawMessage{"hostname":hostJSON,"service":serviceJSON}
			insertAt:=len(rules)
			for i,existing:=range rules { if isCatchall(existing) { insertAt=i;break } }
			rules=append(rules,nil);copy(rules[insertAt+1:],rules[insertAt:]);rules[insertAt]=rule;changed=true
		}
	}
	regular:=make([]map[string]json.RawMessage,0,len(rules))
	catchalls:=make([]map[string]json.RawMessage,0,1)
	for _,rule:=range rules { if isCatchall(rule) { catchalls=append(catchalls,rule) } else { regular=append(regular,rule) } }
	if len(catchalls)>1 { return nil,false,fmt.Errorf("tunnel has multiple catch-all ingress rules; resolve them manually") }
	if len(catchalls)==0 { raw,_:=json.Marshal("http_status:404");catchalls=append(catchalls,map[string]json.RawMessage{"service":raw});changed=true }
	if len(rules)>0 && !isCatchall(rules[len(rules)-1]) && len(catchalls)==1 { changed=true }
	return append(regular,catchalls[0]),changed,nil
}

func cloudflareTunnelConfigPayload(current cloudflareTunnelConfig, rules []map[string]json.RawMessage) map[string]interface{} {
	config:=map[string]interface{}{"ingress":rules}
	if len(current.Config.OriginRequest)>0 { config["originRequest"]=current.Config.OriginRequest }
	return map[string]interface{}{"config":config}
}

func updateCloudflareTunnelIngress(ctx context.Context, token, accountID, tunnelID, hostname, service string, remove, allowReplace bool) error {
	if !validCloudflareAccountID(accountID)||!validCloudflareTunnelID(tunnelID)||!domainPattern.MatchString(hostname){return fmt.Errorf("invalid Cloudflare tunnel route parameters")}
	var tunnel struct{ID string `json:"id"`;AccountTag string `json:"account_tag"`;ConfigSrc string `json:"config_src"`}
	if err:=cloudflareAPIRequest(ctx,token,http.MethodGet,"/accounts/"+accountID+"/cfd_tunnel/"+tunnelID,nil,&tunnel);err!=nil{return err}
	if tunnel.ID!=tunnelID||tunnel.AccountTag!=accountID||tunnel.ConfigSrc!="cloudflare"{return fmt.Errorf("tunnel must be a remotely managed tunnel in the configured account")}
	var current cloudflareTunnelConfig
	if err:=cloudflareAPIRequest(ctx,token,http.MethodGet,"/accounts/"+accountID+"/cfd_tunnel/"+tunnelID+"/configurations",nil,&current);err!=nil{return err}
	if current.Source!="cloudflare"{return fmt.Errorf("tunnel configuration is not remotely managed")}
	rules,changed,err:=updateCloudflareIngressRules(current.Config.Ingress,hostname,service,remove,allowReplace)
	if err!=nil{return err};if !changed{return nil}
	body:=cloudflareTunnelConfigPayload(current,rules)
	return cloudflareAPIRequest(ctx,token,http.MethodPut,"/accounts/"+accountID+"/cfd_tunnel/"+tunnelID+"/configurations",body,nil)
}

func ensureCloudflareTunnelDNS(ctx context.Context, token, zoneID, hostname, tunnelID string) (cloudflareDNSRecord,error) {
	var existing []cloudflareDNSRecord
	endpoint:="/zones/"+zoneID+"/dns_records?type=CNAME&name="+url.QueryEscape(hostname)
	if err:=cloudflareAPIRequest(ctx,token,http.MethodGet,endpoint,nil,&existing);err!=nil{return cloudflareDNSRecord{},err}
	if len(existing)>1{return cloudflareDNSRecord{},fmt.Errorf("multiple DNS records already use this hostname")}
	content:=strings.ToLower(tunnelID)+".cfargotunnel.com"
	payload:=map[string]interface{}{"type":"CNAME","name":hostname,"content":content,"proxied":true,"ttl":1}
	method,endpoint:=http.MethodPost,"/zones/"+zoneID+"/dns_records"
	if len(existing)==1 {
		if existing[0].Type!="CNAME"||!validCloudflareDNSRecordID(existing[0].ID){return cloudflareDNSRecord{},fmt.Errorf("an unmanaged DNS record already uses this hostname")}
		if !strings.EqualFold(existing[0].Content,content){return cloudflareDNSRecord{},fmt.Errorf("DNS record already points to a different tunnel; change it manually before continuing")}
		method=http.MethodPut;endpoint+="/"+existing[0].ID
	}
	var record cloudflareDNSRecord
	if err:=cloudflareAPIRequest(ctx,token,method,endpoint,payload,&record);err!=nil{return cloudflareDNSRecord{},err}
	if !validCloudflareDNSRecordID(record.ID)||record.Name!=hostname||record.Type!="CNAME"||!strings.EqualFold(record.Content,content){return cloudflareDNSRecord{},fmt.Errorf("Cloudflare returned an unexpected DNS record")}
	return record,nil
}

func deleteCloudflareTunnelDNS(ctx context.Context, token, zoneID, hostname, tunnelID string) error {
	var existing []cloudflareDNSRecord
	endpoint:="/zones/"+zoneID+"/dns_records?type=CNAME&name="+url.QueryEscape(hostname)
	if err:=cloudflareAPIRequest(ctx,token,http.MethodGet,endpoint,nil,&existing);err!=nil{return err}
	if len(existing)==0{return nil};if len(existing)>1{return fmt.Errorf("multiple DNS records already use this hostname")}
	record:=existing[0]
	if record.Type!="CNAME"||!validCloudflareDNSRecordID(record.ID)||!strings.EqualFold(record.Content,strings.ToLower(tunnelID)+".cfargotunnel.com"){return fmt.Errorf("DNS record is not managed by this Yevqo tunnel route")}
	return cloudflareAPIRequest(ctx,token,http.MethodDelete,"/zones/"+zoneID+"/dns_records/"+record.ID,nil,nil)
}


func (s *Server) removeCloudflareRouteForSite(ctx context.Context, userID, deviceID int64, siteID string) (bool, string, error) {
	var tunnelID, hostname string
	err:=s.db.conn.QueryRowContext(ctx,`SELECT tunnel_id,hostname FROM site_cloudflare_routes WHERE user_id=? AND device_id=? AND site_id=?`,userID,deviceID,siteID).Scan(&tunnelID,&hostname)
	if err==sql.ErrNoRows{return false,"",nil};if err!=nil{return false,"",err}
	accountID,zoneID,_,token,err:=s.cloudflareCredentials(ctx,userID);if err!=nil{return false,"",err}
	workCtx,cancel:=context.WithTimeout(ctx,30*time.Second);defer cancel()
	if err:=updateCloudflareTunnelIngress(workCtx,token,accountID,tunnelID,hostname,"",true,false);err!=nil{return false,"",err}
	dnsErr:=deleteCloudflareTunnelDNS(workCtx,token,zoneID,hostname,tunnelID)
	if _,err:=s.db.conn.ExecContext(workCtx,`DELETE FROM site_cloudflare_routes WHERE user_id=? AND device_id=? AND site_id=?`,userID,deviceID,siteID);err!=nil{return false,"",err}
	if dnsErr!=nil{return true,"Cloudflare ingress removed; DNS record left unchanged: "+dnsErr.Error(),nil}
	return true,"Cloudflare ingress and managed DNS record removed",nil
}

func (s *Server) handleCloudflareTunnelRoutes(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context())
	if r.Method==http.MethodGet {
		deviceID,err:=strconv.ParseInt(r.URL.Query().Get("device_id"),10,64);if err!=nil||deviceID<1{apiError(w,shared.ErrBadRequest("invalid device id"));return}
		siteID:=r.URL.Query().Get("site_id");if !validBackupSiteID(siteID){apiError(w,shared.ErrBadRequest("invalid site id"));return}
		dev,err:=s.db.GetDeviceByID(r.Context(),deviceID);if err!=nil{apiError(w,shared.ErrNotFound("device not found"));return}
		if err:=s.authorizeWorkspace(r.Context(),dev.UserID,u.ID,"device.read");err!=nil{apiError(w,err);return}
		var tunnelID,hostname string;var port int
		err=s.db.conn.QueryRowContext(r.Context(),`SELECT tunnel_id,hostname,origin_port FROM site_cloudflare_routes WHERE user_id=? AND device_id=? AND site_id=?`,dev.UserID,deviceID,siteID).Scan(&tunnelID,&hostname,&port)
		if err==sql.ErrNoRows{writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"configured":false});return}
		if err!=nil{apiError(w,shared.ErrInternal("could not load Cloudflare route"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"configured":true,"tunnel_id":tunnelID,"hostname":hostname,"origin_port":port,"origin":fmt.Sprintf("http://127.0.0.1:%d",port)});return
	}
	var in struct{DeviceID int64 `json:"device_id"`;SiteID string `json:"site_id"`;TunnelID string `json:"tunnel_id"`}
	if err:=decodeJSON(r,&in);err!=nil||in.DeviceID<1||!validBackupSiteID(in.SiteID){apiError(w,shared.ErrBadRequest("invalid Cloudflare route request"));return}
	dev,err:=s.db.GetDeviceByID(r.Context(),in.DeviceID);if err!=nil{apiError(w,shared.ErrNotFound("device not found"));return}
	if err:=s.authorizeWorkspace(r.Context(),dev.UserID,u.ID,"site.configure");err!=nil{apiError(w,err);return}
	if r.Method==http.MethodPost {
		if u.ID!=dev.UserID{apiError(w,shared.ErrForbidden("only the workspace owner can manage Cloudflare routes"));return}
		if !validCloudflareTunnelID(in.TunnelID){apiError(w,shared.ErrBadRequest("invalid Cloudflare Tunnel ID"));return}
		var hostname string;var port int
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT domain,port FROM sites WHERE device_id=? AND site_id=?`,in.DeviceID,in.SiteID).Scan(&hostname,&port);err!=nil{apiError(w,shared.ErrNotFound("site not found"));return}
		hostname=strings.ToLower(strings.TrimSuffix(strings.TrimSpace(hostname),"."))
		if !domainPattern.MatchString(hostname)||port<1024||port>65535{apiError(w,shared.ErrBadRequest("save a valid site domain and port before configuring Cloudflare"));return}
		accountID,zoneID,zoneName,token,err:=s.cloudflareCredentials(r.Context(),dev.UserID)
		if err!=nil{apiError(w,shared.ErrBadRequest("connect a Cloudflare API token and zone first"));return}
		if !hostnameBelongsToZone(hostname,zoneName){apiError(w,shared.ErrBadRequest("site domain must belong to the connected Cloudflare zone"));return}
		var existingTunnel,existingHost string
		existingErr:=s.db.conn.QueryRowContext(r.Context(),`SELECT tunnel_id,hostname FROM site_cloudflare_routes WHERE user_id=? AND device_id=? AND site_id=?`,dev.UserID,in.DeviceID,in.SiteID).Scan(&existingTunnel,&existingHost)
		if existingErr!=nil&&existingErr!=sql.ErrNoRows{apiError(w,shared.ErrInternal("could not check existing Cloudflare route"));return}
		if existingErr==nil&&(existingTunnel!=in.TunnelID||existingHost!=hostname){apiError(w,shared.ErrConflict("remove the existing Cloudflare route before changing its tunnel or hostname"));return}
		var occupied int
		if err:=s.db.conn.QueryRowContext(r.Context(),`SELECT COUNT(*) FROM site_cloudflare_routes WHERE user_id=? AND hostname=? AND NOT(device_id=? AND site_id=?)`,dev.UserID,hostname,in.DeviceID,in.SiteID).Scan(&occupied);err!=nil{apiError(w,shared.ErrInternal("could not check hostname assignment"));return}
		if occupied>0{apiError(w,shared.ErrConflict("hostname is already assigned to another Yevqo site"));return}
		ctx,cancel:=context.WithTimeout(r.Context(),30*time.Second);defer cancel()
		service:=fmt.Sprintf("http://127.0.0.1:%d",port)
		if err:=updateCloudflareTunnelIngress(ctx,token,accountID,in.TunnelID,hostname,service,false,existingErr==nil);err!=nil{apiError(w,shared.ErrBadRequest("could not configure Cloudflare Tunnel ingress: "+err.Error()));return}
		record,err:=ensureCloudflareTunnelDNS(ctx,token,zoneID,hostname,in.TunnelID)
		if err!=nil{
			rollbackCtx,rollbackCancel:=context.WithTimeout(context.Background(),20*time.Second)
			_ = updateCloudflareTunnelIngress(rollbackCtx,token,accountID,in.TunnelID,hostname,"",true,false)
			rollbackCancel()
			apiError(w,shared.ErrBadRequest("could not create Cloudflare DNS record: "+err.Error()));return
		}
		_,err=s.db.conn.ExecContext(r.Context(),`INSERT INTO site_cloudflare_routes(user_id,device_id,site_id,tunnel_id,hostname,origin_port) VALUES(?,?,?,?,?,?) ON DUPLICATE KEY UPDATE tunnel_id=VALUES(tunnel_id),hostname=VALUES(hostname),origin_port=VALUES(origin_port),updated_at=CURRENT_TIMESTAMP`,dev.UserID,in.DeviceID,in.SiteID,in.TunnelID,hostname,port)
		if err!=nil{
			rollbackCtx,rollbackCancel:=context.WithTimeout(context.Background(),20*time.Second)
			_ = deleteCloudflareTunnelDNS(rollbackCtx,token,zoneID,hostname,in.TunnelID)
			_ = updateCloudflareTunnelIngress(rollbackCtx,token,accountID,in.TunnelID,hostname,"",true)
			rollbackCancel()
			apiError(w,shared.ErrInternal("Cloudflare route was configured but could not be saved"));return
		}
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),DeviceID:shared.PtrInt64(in.DeviceID),Action:"cloudflare_tunnel_route_configured",Details:map[string]interface{}{"site_id":in.SiteID,"hostname":hostname,"tunnel_id":in.TunnelID,"origin_port":port},IP:clientIP(r)})
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"configured":true,"hostname":hostname,"tunnel_id":in.TunnelID,"origin":service,"dns_record":record});return
	}
	if r.Method==http.MethodDelete {
		if u.ID!=dev.UserID{apiError(w,shared.ErrForbidden("only the workspace owner can manage Cloudflare routes"));return}
		removed,message,err:=s.removeCloudflareRouteForSite(r.Context(),dev.UserID,in.DeviceID,in.SiteID)
		if err!=nil{apiError(w,shared.ErrBadRequest("could not remove Cloudflare route: "+err.Error()));return}
		if !removed{writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"configured":false});return}
		_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(u.ID),DeviceID:shared.PtrInt64(in.DeviceID),Action:"cloudflare_tunnel_route_removed",Details:map[string]interface{}{"site_id":in.SiteID},IP:clientIP(r)})
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"configured":false,"message":message});return
	}
	http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
}

func (s *Server) handleCloudflareDNS(w http.ResponseWriter,r *http.Request){
	u:=UserFromContext(r.Context())
	_,zoneID,zoneName,token,err:=s.cloudflareCredentials(r.Context(),u.ID)
	if err!=nil{apiError(w,shared.ErrBadRequest("connect and verify a Cloudflare zone first"));return}
	ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second);defer cancel()
	switch r.Method {
	case http.MethodGet:
		var records []cloudflareDNSRecord
		err:=cloudflareAPIRequest(ctx,token,http.MethodGet,"/zones/"+zoneID+"/dns_records?per_page=100",nil,&records)
		if err!=nil{apiError(w,shared.ErrBadRequest("could not load Cloudflare DNS records"));return}
		managed:=make([]cloudflareDNSRecord,0)
		for _,record:=range records{if record.Type=="CNAME"&&strings.HasSuffix(strings.ToLower(record.Content),".cfargotunnel.com"){managed=append(managed,record)}}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"records":managed,"zone_name":zoneName})
	case http.MethodPost:
		var in struct{Hostname string `json:"hostname"`;TunnelID string `json:"tunnel_id"`}
		if err:=decodeJSON(r,&in);err!=nil{apiError(w,shared.ErrBadRequest("invalid DNS record"));return}
		in.Hostname=strings.ToLower(strings.TrimSuffix(strings.TrimSpace(in.Hostname),"."))
		in.TunnelID=strings.TrimSpace(in.TunnelID)
		if !domainPattern.MatchString(in.Hostname)||!hostnameBelongsToZone(in.Hostname,zoneName)||!validCloudflareTunnelID(in.TunnelID){apiError(w,shared.ErrBadRequest("hostname must belong to the configured zone and tunnel ID must be a UUID"));return}
		var existing []cloudflareDNSRecord
		err:=cloudflareAPIRequest(ctx,token,http.MethodGet,"/zones/"+zoneID+"/dns_records?type=CNAME&name="+in.Hostname,nil,&existing)
		if err!=nil{apiError(w,shared.ErrBadRequest("could not check existing DNS record"));return}
		payload:=map[string]interface{}{"type":"CNAME","name":in.Hostname,"content":strings.ToLower(in.TunnelID)+".cfargotunnel.com","proxied":true,"ttl":1}
		method,endpoint:=http.MethodPost,"/zones/"+zoneID+"/dns_records"
		if len(existing)>1 {
			apiError(w,shared.ErrConflict("multiple DNS records already use this hostname; resolve the duplicate records manually"))
			return
		}
		if len(existing)==1 {
			if existing[0].Type!="CNAME"||!strings.HasSuffix(strings.ToLower(existing[0].Content),".cfargotunnel.com")||!validCloudflareDNSRecordID(existing[0].ID) {
				apiError(w,shared.ErrBadRequest("an unmanaged DNS record already uses this hostname; refusing to overwrite it"))
				return
			}
			method=http.MethodPut;endpoint+="/"+existing[0].ID
		}
		var record cloudflareDNSRecord
		if err:=cloudflareAPIRequest(ctx,token,method,endpoint,payload,&record);err!=nil{apiError(w,shared.ErrBadRequest("Cloudflare could not create the tunnel DNS record"));return}
		if !validCloudflareDNSRecordID(record.ID)||record.Name!=in.Hostname||record.Type!="CNAME"||!strings.EqualFold(record.Content,payload["content"].(string)) {
			apiError(w,shared.ErrInternal("Cloudflare returned an unexpected DNS record"))
			return
		}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"record":record})
	case http.MethodDelete:
		var in struct{RecordID string `json:"record_id"`}
		if err:=decodeJSON(r,&in);err!=nil||!validCloudflareDNSRecordID(in.RecordID){apiError(w,shared.ErrBadRequest("invalid DNS record id"));return}
		var record cloudflareDNSRecord
		if err:=cloudflareAPIRequest(ctx,token,http.MethodGet,"/zones/"+zoneID+"/dns_records/"+in.RecordID,nil,&record);err!=nil||record.ID!=in.RecordID||record.Type!="CNAME"||!strings.HasSuffix(strings.ToLower(record.Content),".cfargotunnel.com")||!hostnameBelongsToZone(record.Name,zoneName){apiError(w,shared.ErrNotFound("DNS record not found in configured zone"));return}
		if err:=cloudflareAPIRequest(ctx,token,http.MethodDelete,"/zones/"+zoneID+"/dns_records/"+in.RecordID,nil,nil);err!=nil{apiError(w,shared.ErrBadRequest("could not delete DNS record"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
	default:http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}
