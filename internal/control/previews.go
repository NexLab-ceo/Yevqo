package control

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"veltrix/internal/shared"
)

type previewCreateInput struct {
	Branch string `json:"branch"`
	Commit string `json:"commit"`
	AllowProjectCode bool `json:"allow_project_code"`
}

func newPreviewKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil { return "", err }
	return hex.EncodeToString(b), nil
}

func previewPort() (uint32, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil { return 0, err }
	return 30000 + uint32((uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]))%20000), nil
}

func (s *Server) handleSitePreviews(w http.ResponseWriter, r *http.Request, user *shared.User, deviceID int64, siteID, previewKey string) {
	dev, err := s.db.GetDeviceByID(r.Context(), deviceID)
	if err != nil { apiError(w, shared.ErrNotFound("device not found")); return }
	if !strings.EqualFold(strings.TrimSpace(dev.Platform), "android") {
		apiError(w, shared.ErrForbidden("Preview Deployments are available only on Android/Termux devices"))
		return
	}
	if err := s.authorizeWorkspace(r.Context(), dev.UserID, user.ID, "device.read"); err != nil { apiError(w, err); return }

	sites, err := s.db.ListSitesByDevice(r.Context(), deviceID)
	if err != nil { apiError(w, shared.ErrInternal("could not load site")); return }
	var site *shared.Site
	for i := range sites { if sites[i].SiteID == siteID { site = &sites[i]; break } }
	if site == nil { apiError(w, shared.ErrNotFound("site not found")); return }

	if previewKey == "" {
		switch r.Method {
		case http.MethodGet:
			rows, err := s.db.conn.QueryContext(r.Context(), `SELECT preview_key,source_branch,source_commit,runtime,port,pid,status,error_message,expires_at,created_at FROM preview_deployments WHERE user_id=? AND device_id=? AND site_id=? ORDER BY created_at DESC`, user.ID, deviceID, siteID)
			if err != nil { apiError(w, shared.ErrInternal("could not list previews")); return }
			defer rows.Close()
			type item struct { Key string `json:"key"`; Branch string `json:"branch"`; Commit string `json:"commit"`; Runtime string `json:"runtime"`; Port uint32 `json:"port"`; PID *uint32 `json:"pid,omitempty"`; Status string `json:"status"`; Error string `json:"error,omitempty"`; ExpiresAt *time.Time `json:"expires_at,omitempty"`; CreatedAt time.Time `json:"created_at"`; URL string `json:"url,omitempty"` }
			items := []item{}
			for rows.Next() { var v item; if err := rows.Scan(&v.Key,&v.Branch,&v.Commit,&v.Runtime,&v.Port,&v.PID,&v.Status,&v.Error,&v.ExpiresAt,&v.CreatedAt); err != nil { apiError(w, shared.ErrInternal("could not read previews")); return }; v.URL = fmt.Sprintf("http://127.0.0.1:%d",v.Port); items=append(items,v) }
			if err := rows.Err(); err != nil { apiError(w, shared.ErrInternal("could not read previews")); return }
			writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"previews":items})
			return
		case http.MethodPost:
			if err := s.authorizeWorkspace(r.Context(), dev.UserID, user.ID, "preview.deploy"); err != nil { apiError(w, err); return }
			var in previewCreateInput
			if err := decodeJSON(r,&in); err != nil { apiError(w,shared.ErrBadRequest("invalid JSON")); return }
			if !in.AllowProjectCode { apiError(w,shared.ErrBadRequest("explicit project-code consent is required before creating a preview")); return }
			cfg, err := s.siteGitSettings(r.Context(),deviceID,siteID)
			if err != nil || strings.TrimSpace(cfg.GitURL)=="" { apiError(w,shared.ErrBadRequest("connect a Git repository before creating a preview")); return }
			branch:=strings.TrimSpace(in.Branch); if branch=="" { branch=cfg.GitBranch }; if branch=="" { branch="main" }
			if !gitBranchPattern.MatchString(branch) || strings.Contains(branch,"..") || strings.Contains(branch,"@{") || strings.HasSuffix(branch,"/") || strings.HasSuffix(branch,".") { apiError(w,shared.ErrBadRequest("invalid Git branch")); return }
			if in.Commit!="" && !releaseCommitPattern.MatchString(in.Commit) { apiError(w,shared.ErrBadRequest("invalid Git commit SHA")); return }
			if dev.Status != shared.DeviceOnline { apiError(w,shared.ErrBadRequest("Android device is offline")); return }
			conn,online:=s.hub.Get(deviceID); if !online { apiError(w,shared.ErrBadRequest("Android device is offline")); return }
			key,err:=newPreviewKey(); if err!=nil { apiError(w,shared.ErrInternal("could not allocate preview ID")); return }
			port,err:=previewPort(); if err!=nil { apiError(w,shared.ErrInternal("could not allocate preview port")); return }
			previewID:="preview_"+key
			_,err=s.db.conn.ExecContext(r.Context(),`INSERT INTO preview_deployments(user_id,device_id,site_id,preview_key,source_branch,source_commit,runtime,path,port,status,expires_at) VALUES(?,?,?,?,?,?,?,?,?,'building',DATE_ADD(UTC_TIMESTAMP(),INTERVAL 24 HOUR))`,user.ID,deviceID,siteID,key,branch,in.Commit,site.Runtime,"sites/"+previewID,port)
			if err!=nil { apiError(w,shared.ErrInternal("could not reserve preview")); return }
			action:=shared.PreviewDeploy{PreviewID:previewID,SiteAction:shared.SiteAction{SiteId:previewID,Path:"sites/"+previewID,Port:port,Runtime:site.Runtime,GitURL:cfg.GitURL,GitBranch:branch,GitCommit:in.Commit,BuildCommand:cfg.BuildCommand,RootDir:cfg.RootDir,GitToken:cfg.GitToken,HealthCheckPath:cfg.HealthCheckPath,CloneTimeoutSeconds:cfg.CloneTimeoutSeconds,BuildTimeoutSeconds:cfg.BuildTimeoutSeconds,HealthCheckTimeoutSeconds:cfg.HealthCheckTimeoutSeconds,StartCommand:cfg.StartCommand,AllowProjectCode:true}}
			ctx,cancel:=context.WithTimeout(r.Context(),25*time.Minute); defer cancel()
			result,err:=conn.SendCommand(ctx,"preview_deploy",&action)
			if err!=nil || result==nil || !result.Ok {
				msg:="preview deployment failed"
				if result!=nil&&result.Message!="" { msg=result.Message }
				// The request may have timed out while the Agent was still deploying.
				// Use an independent bounded context to avoid leaving an orphan process.
				cleanupCtx,cleanupCancel:=context.WithTimeout(context.Background(),30*time.Second)
				_,_=conn.SendCommand(cleanupCtx,"preview_remove",&shared.PreviewRemove{PreviewID:previewID})
				cleanupCancel()
				_,_=s.db.conn.ExecContext(r.Context(),`UPDATE preview_deployments SET status='failed',error_message=? WHERE device_id=? AND site_id=? AND preview_key=?`,truncatePreviewError(msg),deviceID,siteID,key)
				writeJSON(w,http.StatusOK,map[string]interface{}{"ok":false,"key":key,"status":"failed","message":msg})
				return
			}
			var started shared.StartSiteResult; if err:=shared.Unmarshal(result.Payload,&started); err!=nil { _,_=conn.SendCommand(r.Context(),"preview_remove",&shared.PreviewRemove{PreviewID:previewID}); _,_=s.db.conn.ExecContext(r.Context(),`UPDATE preview_deployments SET status='failed',error_message='invalid Agent response' WHERE device_id=? AND site_id=? AND preview_key=?`,deviceID,siteID,key); apiError(w,shared.ErrInternal("invalid preview response")); return }
			_,err=s.db.conn.ExecContext(r.Context(),`UPDATE preview_deployments SET status='ready',pid=? WHERE device_id=? AND site_id=? AND preview_key=?`,started.Pid,deviceID,siteID,key)
			if err!=nil { _,_=conn.SendCommand(r.Context(),"preview_remove",&shared.PreviewRemove{PreviewID:previewID}); apiError(w,shared.ErrInternal("could not save preview status")); return }
			url:=fmt.Sprintf("http://127.0.0.1:%d",port)
			_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(user.ID),DeviceID:shared.PtrInt64(deviceID),Action:"preview_create",Details:map[string]interface{}{"site_id":siteID,"preview_key":key,"branch":branch},IP:clientIP(r)})
			writeJSON(w,http.StatusCreated,map[string]interface{}{"ok":true,"key":key,"status":"ready","port":port,"pid":started.Pid,"commit":started.CommitHash,"url":url,"expires_in_hours":24})
			return
		default: http.Error(w,"method not allowed",http.StatusMethodNotAllowed); return
		}
	}

	if r.Method != http.MethodDelete { http.Error(w,"method not allowed",http.StatusMethodNotAllowed); return }
	if err := s.authorizeWorkspace(r.Context(),dev.UserID,user.ID,"preview.delete"); err != nil { apiError(w,err); return }
	if len(previewKey)!=32 { apiError(w,shared.ErrBadRequest("invalid preview key")); return }
	var status string
	err=s.db.conn.QueryRowContext(r.Context(),`SELECT status FROM preview_deployments WHERE user_id=? AND device_id=? AND site_id=? AND preview_key=?`,user.ID,deviceID,siteID,previewKey).Scan(&status)
	if err!=nil { apiError(w,shared.ErrNotFound("preview not found")); return }
	conn,online:=s.hub.Get(deviceID); if !online { apiError(w,shared.ErrBadRequest("reconnect the Android device before deleting this preview")); return }
	ctx,cancel:=context.WithTimeout(r.Context(),30*time.Second); defer cancel()
	result,err:=conn.SendCommand(ctx,"preview_remove",&shared.PreviewRemove{PreviewID:"preview_"+previewKey})
	if err!=nil || result==nil || !result.Ok { msg:="could not remove preview from Android device"; if result!=nil&&result.Message!="" { msg=result.Message }; apiError(w,shared.ErrBadRequest(msg)); return }
	if _,err:=s.db.conn.ExecContext(r.Context(),`DELETE FROM preview_deployments WHERE user_id=? AND device_id=? AND site_id=? AND preview_key=?`,user.ID,deviceID,siteID,previewKey); err!=nil { apiError(w,shared.ErrInternal("could not delete preview record")); return }
	_ = s.db.Log(r.Context(),&shared.LogEntry{UserID:shared.PtrInt64(user.ID),DeviceID:shared.PtrInt64(deviceID),Action:"preview_delete",Details:map[string]interface{}{"site_id":siteID,"preview_key":previewKey},IP:clientIP(r)})
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"message":"preview removed"})
}

func truncatePreviewError(s string) string { s=strings.TrimSpace(s); if len(s)>900 { return s[:900] }; return s }


func (s *Server) runDuePreviewExpiry() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	rows, err := s.db.conn.QueryContext(ctx, `SELECT id,device_id,site_id,preview_key FROM preview_deployments WHERE expires_at IS NOT NULL AND expires_at<=UTC_TIMESTAMP() AND status NOT IN ('expired','stopped') ORDER BY expires_at LIMIT 50`)
	if err != nil { return }
	type expiredPreview struct { id, deviceID int64; siteID, key string }
	items := make([]expiredPreview,0)
	for rows.Next() { var item expiredPreview; if rows.Scan(&item.id,&item.deviceID,&item.siteID,&item.key)==nil { items=append(items,item) } }
	rows.Close()
	for _, item := range items {
		conn, online := s.hub.Get(item.deviceID)
		if !online { continue } // Keep retrying until the Android Agent reconnects.
		cmdCtx, cmdCancel := context.WithTimeout(ctx, 30*time.Second)
		result, sendErr := conn.SendCommand(cmdCtx,"preview_remove",&shared.PreviewRemove{PreviewID:"preview_"+item.key})
		cmdCancel()
		if sendErr != nil || result == nil || !result.Ok { continue }
		_, _ = s.db.conn.ExecContext(ctx,`UPDATE preview_deployments SET status='expired',pid=NULL,error_message='' WHERE id=? AND expires_at<=UTC_TIMESTAMP()`,item.id)
	}
}

