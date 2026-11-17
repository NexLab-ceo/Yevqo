package control

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

type backupSchedule struct {
	ID int64 `json:"id"`
	ResourceType string `json:"resource_type"`
	DeviceID int64 `json:"device_id"`
	SiteID string `json:"site_id,omitempty"`
	DatabaseID int64 `json:"database_id,omitempty"`
	IntervalHours int `json:"interval_hours"`
	RetentionCount int `json:"retention_count"`
	Enabled bool `json:"enabled"`
	NextRunAt time.Time `json:"next_run_at"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	LastStatus string `json:"last_status"`
	LastError string `json:"last_error,omitempty"`
}

func (s *Server) handleBackupSchedules(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if r.Method == http.MethodGet {
		rows, err := s.db.conn.QueryContext(r.Context(), `SELECT id,resource_type,device_id,COALESCE(site_id,''),COALESCE(database_id,0),interval_hours,retention_count,enabled,next_run_at,last_run_at,last_status,COALESCE(last_error,'') FROM backup_schedules WHERE user_id=? ORDER BY created_at DESC`, u.ID)
		if err != nil { apiError(w, shared.ErrInternal("could not load backup schedules")); return }
		defer rows.Close()
		out := make([]backupSchedule, 0)
		for rows.Next() {
			var item backupSchedule
			if err := rows.Scan(&item.ID,&item.ResourceType,&item.DeviceID,&item.SiteID,&item.DatabaseID,&item.IntervalHours,&item.RetentionCount,&item.Enabled,&item.NextRunAt,&item.LastRunAt,&item.LastStatus,&item.LastError); err != nil { apiError(w, shared.ErrInternal("could not read backup schedules")); return }
			out = append(out, item)
		}
		if err := rows.Err(); err != nil { apiError(w, shared.ErrInternal("could not read backup schedules")); return }
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true,"schedules":out}); return
	}
	if r.Method == http.MethodPost {
		var in struct { ResourceType string `json:"resource_type"`; DeviceID int64 `json:"device_id"`; SiteID string `json:"site_id"`; DatabaseID int64 `json:"database_id"`; IntervalHours int `json:"interval_hours"`; RetentionCount int `json:"retention_count"` }
		if err := decodeJSON(r, &in); err != nil { apiError(w, shared.ErrBadRequest("invalid schedule")); return }
		if in.IntervalHours < 6 || in.IntervalHours > 8760 { apiError(w, shared.ErrBadRequest("interval must be between 6 hours and 365 days")); return }
		if in.RetentionCount < 1 || in.RetentionCount > 50 { apiError(w, shared.ErrBadRequest("retention must be between 1 and 50 backups")); return }
		var owned int
		if in.ResourceType == "site" {
			if !validBackupSiteID(in.SiteID) || in.DeviceID < 1 || in.DatabaseID != 0 { apiError(w, shared.ErrBadRequest("invalid website schedule target")); return }
			if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM sites st JOIN devices d ON d.id=st.device_id WHERE st.device_id=? AND st.site_id=? AND d.user_id=?`, in.DeviceID,in.SiteID,u.ID).Scan(&owned); err != nil || owned != 1 { apiError(w, shared.ErrNotFound("website not found")); return }
		} else if in.ResourceType == "database" {
			if in.DatabaseID < 1 { apiError(w, shared.ErrBadRequest("database id is required")); return }
			if err := s.db.conn.QueryRowContext(r.Context(), `SELECT ds.device_id FROM db_instances db JOIN database_servers ds ON ds.id=db.server_id WHERE db.id=? AND db.user_id=? AND db.status='ready' AND ds.status='ready'`, in.DatabaseID,u.ID).Scan(&in.DeviceID); err != nil || in.DeviceID < 1 { apiError(w, shared.ErrNotFound("ready database not found")); return }
		} else { apiError(w, shared.ErrBadRequest("resource_type must be site or database")); return }
		res, err := s.db.conn.ExecContext(r.Context(), `INSERT INTO backup_schedules(user_id,device_id,resource_type,site_id,database_id,interval_hours,retention_count,next_run_at) VALUES(?,?,?,?,?,?,?,DATE_ADD(UTC_TIMESTAMP(),INTERVAL ? HOUR))`, u.ID,in.DeviceID,in.ResourceType,nullableString(in.SiteID),nullablePositiveID(in.DatabaseID),in.IntervalHours,in.RetentionCount,in.IntervalHours)
		if err != nil { apiError(w, shared.ErrInternal("could not create backup schedule")); return }
		id,_ := res.LastInsertId()
		_ = s.db.Log(r.Context(), &shared.LogEntry{UserID:shared.PtrInt64(u.ID),DeviceID:shared.PtrInt64(in.DeviceID),Action:"backup_schedule_created",Details:map[string]interface{}{"schedule_id":id,"resource_type":in.ResourceType}})
		writeJSON(w,http.StatusCreated,map[string]interface{}{"ok":true,"schedule_id":id}); return
	}
	if r.Method == http.MethodDelete {
		raw := strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/backup-schedules/"),"/")
		id, err := strconv.ParseInt(raw,10,64)
		if err != nil || id < 1 { apiError(w,shared.ErrBadRequest("invalid schedule id")); return }
		res, err := s.db.conn.ExecContext(r.Context(), `DELETE FROM backup_schedules WHERE id=? AND user_id=?`,id,u.ID)
		if err != nil { apiError(w,shared.ErrInternal("could not delete backup schedule")); return }
		n,_ := res.RowsAffected(); if n == 0 { apiError(w,shared.ErrNotFound("schedule not found")); return }
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true}); return
	}
	http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
}

func nullablePositiveID(id int64) interface{} { if id > 0 { return id }; return nil }

func (s *Server) backupSchedulerLoop() {
	defer close(s.schedulerDone)
	ticker := time.NewTicker(30*time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.schedulerStop: return
		case <-ticker.C: s.runDueBackupSchedules()
		}
	}
}

type dueBackupSchedule struct { ID,UserID,DeviceID,DatabaseID int64; ResourceType,SiteID string; IntervalHours,Retention int }

func (s *Server) runDueBackupSchedules() {
	ctx,cancel := context.WithTimeout(context.Background(),20*time.Second); defer cancel()
	rows,err := s.db.conn.QueryContext(ctx,`SELECT id,user_id,device_id,resource_type,COALESCE(site_id,''),COALESCE(database_id,0),interval_hours,retention_count FROM backup_schedules WHERE enabled=TRUE AND next_run_at<=UTC_TIMESTAMP() ORDER BY next_run_at LIMIT 25`)
	if err != nil { return }
	items := make([]dueBackupSchedule,0)
	for rows.Next() { var item dueBackupSchedule; if rows.Scan(&item.ID,&item.UserID,&item.DeviceID,&item.ResourceType,&item.SiteID,&item.DatabaseID,&item.IntervalHours,&item.Retention)==nil { items=append(items,item) } }
	rows.Close()
	for _,item := range items { s.queueScheduledBackup(item) }
}

func (s *Server) queueScheduledBackup(item dueBackupSchedule) {
	ctx,cancel := context.WithTimeout(context.Background(),10*time.Second); defer cancel()
	claimed,err := s.db.conn.ExecContext(ctx,`UPDATE backup_schedules SET next_run_at=DATE_ADD(UTC_TIMESTAMP(),INTERVAL ? HOUR),last_run_at=UTC_TIMESTAMP(),last_status='queued',last_error=NULL WHERE id=? AND enabled=TRUE AND next_run_at<=UTC_TIMESTAMP()`,item.IntervalHours,item.ID)
	if err != nil { return }; n,_ := claimed.RowsAffected(); if n != 1 { return }
	conn,online := s.hub.Get(item.DeviceID)
	if !online { s.failBackupSchedule(item,"device is offline"); return }
	if item.ResourceType == "site" {
		resourceID := fmt.Sprintf("%d:%s",item.DeviceID,item.SiteID)
		res,err := s.db.conn.ExecContext(ctx,`INSERT INTO backups(user_id,device_id,resource_type,resource_id,file_name,size_bytes,status) VALUES(?,?, 'site', ?, ?,0,'queued')`,item.UserID,item.DeviceID,resourceID,"scheduled-"+item.SiteID+".tar.gz")
		if err != nil { s.failBackupSchedule(item,"could not queue site backup"); return }
		id,_ := res.LastInsertId()
		go func(){ s.runSiteBackup(id,item.UserID,item.DeviceID,conn,&shared.CreateSiteBackup{SiteId:item.SiteID}); if err:=s.applyScheduledBackupRetention(item,id); err!=nil { s.failBackupSchedule(item,"backup created but retention failed: "+err.Error()); return }; s.finishScheduledBackup(item,id) }()
		return
	}
	var serverID int64; var database string; var port int; var engine,status,serverStatus string
	err = s.db.conn.QueryRowContext(ctx,`SELECT ds.id,db.name,ds.port,ds.engine,db.status,ds.status FROM db_instances db JOIN database_servers ds ON ds.id=db.server_id WHERE db.id=? AND db.user_id=? AND ds.device_id=?`,item.DatabaseID,item.UserID,item.DeviceID).Scan(&serverID,&database,&port,&engine,&status,&serverStatus)
	if err != nil || engine!="postgresql" || status!="ready" || serverStatus!="ready" { s.failBackupSchedule(item,"database or PostgreSQL server is not ready"); return }
	res,err := s.db.conn.ExecContext(ctx,`INSERT INTO backups(user_id,device_id,database_server_id,resource_type,resource_id,file_name,size_bytes,status) VALUES(?,?,?,'database',?,?,0,'queued')`,item.UserID,item.DeviceID,serverID,fmt.Sprint(item.DatabaseID),fmt.Sprintf("scheduled-%d.dump",item.DatabaseID))
	if err != nil { s.failBackupSchedule(item,"could not queue database backup"); return }
	id,_ := res.LastInsertId()
	go func(){ s.runDatabaseBackup(id,item.UserID,item.DeviceID,conn,&shared.CreateDatabaseBackup{DatabaseID:item.DatabaseID,ServerID:serverID,Database:database,Port:uint32(port)}); if err:=s.applyScheduledBackupRetention(item,id); err!=nil { s.failBackupSchedule(item,"backup created but retention failed: "+err.Error()); return }; s.finishScheduledBackup(item,id) }()
}

func (s *Server) applyScheduledBackupRetention(item dueBackupSchedule, backupID int64) error {
	var backupStatus string
	if err:=s.db.conn.QueryRowContext(context.Background(),`SELECT status FROM backups WHERE id=? AND user_id=?`,backupID,item.UserID).Scan(&backupStatus);err!=nil{return err}
	if item.ResourceType=="site" && backupStatus!="completed" { return fmt.Errorf("new site backup did not complete") }
	if item.ResourceType=="database" && backupStatus!="verified" { return fmt.Errorf("new database backup was not verified") }
	conn,online:=s.hub.Get(item.DeviceID)
	if !online { return fmt.Errorf("device went offline before retention could run") }
	req:=&shared.PruneBackups{Keep:item.Retention}
	resourceType,resourceID:="site",fmt.Sprintf("%d:%s",item.DeviceID,item.SiteID)
	if item.ResourceType=="site" { req.SiteId=item.SiteID } else {
		var serverID int64
		if err:=s.db.conn.QueryRowContext(context.Background(),`SELECT server_id FROM db_instances WHERE id=? AND user_id=?`,item.DatabaseID,item.UserID).Scan(&serverID);err!=nil{return err}
		req.SiteId="";req.ServerID=serverID
		resourceType,resourceID="database",fmt.Sprint(item.DatabaseID)
	}
	ctx,cancel:=context.WithTimeout(context.Background(),45*time.Second);defer cancel()
	result,err:=conn.SendCommand(ctx,"prune_backups",req)
	if err!=nil{return err};if !result.Ok{return fmt.Errorf("%s",result.Message)}
	var pruned shared.PruneBackupsResult
	if err:=shared.Unmarshal(result.Payload,&pruned);err!=nil{return err}
	for _,name:=range pruned.Removed {
		if len(name)>255 || strings.Contains(name,"/") || strings.Contains(name,"\\") { continue }
		_,_ = s.db.conn.ExecContext(context.Background(),`DELETE FROM backups WHERE user_id=? AND device_id=? AND resource_type=? AND resource_id=? AND file_name=?`,item.UserID,item.DeviceID,resourceType,resourceID,name)
	}
	return nil
}

func (s *Server) finishScheduledBackup(item dueBackupSchedule, backupID int64) {
	ctx,cancel := context.WithTimeout(context.Background(),5*time.Second); defer cancel()
	var status string
	if err:=s.db.conn.QueryRowContext(ctx,`SELECT status FROM backups WHERE id=? AND user_id=?`,backupID,item.UserID).Scan(&status);err!=nil{return}
	if status=="failed" { s.failBackupSchedule(item,"backup job failed"); return }
	_,_ = s.db.conn.ExecContext(ctx,`UPDATE backup_schedules SET last_status='completed',last_error=NULL WHERE id=?`,item.ID)
	_,_ = s.db.conn.ExecContext(ctx,`INSERT INTO notifications(user_id,type,title,body) VALUES(?,?,?,?)`,item.UserID,"success","Scheduled backup completed",fmt.Sprintf("Backup #%d completed. Retention target: %d.",backupID,item.Retention))
}

func (s *Server) failBackupSchedule(item dueBackupSchedule, reason string) {
	_,_ = s.db.conn.ExecContext(context.Background(),`UPDATE backup_schedules SET last_status='failed',last_error=? WHERE id=?`,reason,item.ID)
	_,_ = s.db.conn.ExecContext(context.Background(),`INSERT INTO notifications(user_id,type,title,body) VALUES(?,?,?,?)`,item.UserID,"error","Scheduled backup failed",reason)
}
