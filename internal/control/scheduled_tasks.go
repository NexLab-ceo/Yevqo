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

type scheduledTaskView struct {
	ID int64 `json:"id"`
	DeviceID int64 `json:"device_id"`
	SiteID string `json:"site_id"`
	Name string `json:"name"`
	Command string `json:"command"`
	IntervalMinutes int `json:"interval_minutes"`
	Enabled bool `json:"enabled"`
	NextRunAt time.Time `json:"next_run_at"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	LastStatus string `json:"last_status"`
	LastOutput string `json:"last_output,omitempty"`
	LastError string `json:"last_error,omitempty"`
}

func (s *Server) handleScheduledTasks(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if r.Method == http.MethodGet {
		deviceID, err := strconv.ParseInt(r.URL.Query().Get("device_id"), 10, 64)
		if err != nil || deviceID < 1 { apiError(w, shared.ErrBadRequest("device_id is required")); return }
		siteID := strings.TrimSpace(r.URL.Query().Get("site_id"))
		if !validBackupSiteID(siteID) { apiError(w, shared.ErrBadRequest("invalid site_id")); return }
		ownerID, accessErr := s.authorizeDeviceOwner(r.Context(), deviceID, u.ID, "device.read")
		if accessErr != nil { apiError(w, accessErr); return }
		if err := s.authorizeWorkspace(r.Context(), ownerID, u.ID, "device.read"); err != nil { apiError(w, err); return }
		var owned int
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM sites WHERE device_id=? AND site_id=?`, deviceID, siteID).Scan(&owned); err != nil || owned != 1 {
			apiError(w, shared.ErrNotFound("site not found")); return
		}
		rows, err := s.db.conn.QueryContext(r.Context(), `SELECT id,device_id,site_id,name,command,interval_minutes,enabled,next_run_at,last_run_at,last_status,COALESCE(last_output,''),COALESCE(last_error,'') FROM scheduled_tasks WHERE user_id=? AND device_id=? AND site_id=? ORDER BY created_at DESC`, ownerID, deviceID, siteID)
		if err != nil { apiError(w, shared.ErrInternal("could not load scheduled tasks")); return }
		defer rows.Close()
		out := make([]scheduledTaskView, 0)
		for rows.Next() {
			var item scheduledTaskView
			if err := rows.Scan(&item.ID,&item.DeviceID,&item.SiteID,&item.Name,&item.Command,&item.IntervalMinutes,&item.Enabled,&item.NextRunAt,&item.LastRunAt,&item.LastStatus,&item.LastOutput,&item.LastError); err != nil { apiError(w, shared.ErrInternal("could not read scheduled tasks")); return }
			out = append(out, item)
		}
		if err := rows.Err(); err != nil { apiError(w, shared.ErrInternal("could not read scheduled tasks")); return }
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true,"tasks":out})
		return
	}
	if r.Method == http.MethodPost {
		var in struct { DeviceID int64 `json:"device_id"`; SiteID string `json:"site_id"`; Name string `json:"name"`; Command string `json:"command"`; IntervalMinutes int `json:"interval_minutes"` }
		if err := decodeJSON(r, &in); err != nil { apiError(w, shared.ErrBadRequest("invalid scheduled task")); return }
		in.Name, in.Command = strings.TrimSpace(in.Name), strings.TrimSpace(in.Command)
		if in.DeviceID < 1 || !validBackupSiteID(in.SiteID) || len(in.Name) < 1 || len(in.Name) > 100 || len(in.Command) < 1 || len(in.Command) > 4096 || strings.IndexByte(in.Command, 0) >= 0 || in.IntervalMinutes < 15 || in.IntervalMinutes > 43200 {
			apiError(w, shared.ErrBadRequest("name, command and interval (15 minutes–30 days) are required")); return
		}
		ownerID, accessErr := s.authorizeDeviceOwner(r.Context(), in.DeviceID, u.ID, "site.configure")
		if accessErr != nil { apiError(w, accessErr); return }
		var owned int
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM sites WHERE device_id=? AND site_id=?`, in.DeviceID, in.SiteID).Scan(&owned); err != nil || owned != 1 {
			apiError(w, shared.ErrNotFound("site not found")); return
		}
		res, err := s.db.conn.ExecContext(r.Context(), `INSERT INTO scheduled_tasks(user_id,device_id,site_id,name,command,interval_minutes,next_run_at) VALUES(?,?,?,?,?,?,DATE_ADD(UTC_TIMESTAMP(),INTERVAL ? MINUTE))`, ownerID,in.DeviceID,in.SiteID,in.Name,in.Command,in.IntervalMinutes,in.IntervalMinutes)
		if err != nil { apiError(w, shared.ErrInternal("could not create scheduled task")); return }
		id, _ := res.LastInsertId()
		_ = s.db.Log(r.Context(), &shared.LogEntry{UserID:shared.PtrInt64(u.ID),DeviceID:shared.PtrInt64(in.DeviceID),Action:"scheduled_task_created",Details:map[string]interface{}{"task_id":id,"site_id":in.SiteID}})
		writeJSON(w, http.StatusCreated, map[string]interface{}{"ok":true,"task_id":id})
		return
	}
	if r.Method == http.MethodPatch || r.Method == http.MethodDelete {
		raw := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/scheduled-tasks/"), "/")
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 1 { apiError(w, shared.ErrBadRequest("invalid task id")); return }
		var ownerID int64
		if err := s.db.conn.QueryRowContext(r.Context(), `SELECT user_id FROM scheduled_tasks WHERE id=?`, id).Scan(&ownerID); err != nil { apiError(w, shared.ErrNotFound("task not found")); return }
		if err := s.authorizeWorkspace(r.Context(), ownerID, u.ID, "site.configure"); err != nil { apiError(w, err); return }
		if r.Method == http.MethodDelete {
			res, err := s.db.conn.ExecContext(r.Context(), `DELETE FROM scheduled_tasks WHERE id=? AND user_id=?`, id, ownerID)
			if err != nil { apiError(w, shared.ErrInternal("could not delete task")); return }
			n, _ := res.RowsAffected(); if n == 0 { apiError(w, shared.ErrNotFound("task not found")); return }
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true}); return
		}
		var in struct { Enabled bool `json:"enabled"` }
		if err := decodeJSON(r, &in); err != nil { apiError(w, shared.ErrBadRequest("invalid task update")); return }
		res, err := s.db.conn.ExecContext(r.Context(), `UPDATE scheduled_tasks SET enabled=?,next_run_at=IF(?,DATE_ADD(UTC_TIMESTAMP(),INTERVAL interval_minutes MINUTE),next_run_at) WHERE id=? AND user_id=?`, in.Enabled,in.Enabled,id,ownerID)
		if err != nil { apiError(w, shared.ErrInternal("could not update task")); return }
		n, _ := res.RowsAffected(); if n == 0 { apiError(w, shared.ErrNotFound("task not found")); return }
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok":true,"enabled":in.Enabled}); return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

type dueScheduledTask struct{ID,UserID,DeviceID int64;SiteID,Name,Command string;IntervalMinutes int}

func (s *Server) runDueScheduledTasks(){
	ctx,cancel:=context.WithTimeout(context.Background(),15*time.Second);defer cancel()
	rows,err:=s.db.conn.QueryContext(ctx,`SELECT id,user_id,device_id,site_id,name,command,interval_minutes FROM scheduled_tasks WHERE enabled=TRUE AND next_run_at<=UTC_TIMESTAMP() ORDER BY next_run_at LIMIT 25`)
	if err!=nil{return}
	items:=make([]dueScheduledTask,0)
	for rows.Next(){var item dueScheduledTask;if rows.Scan(&item.ID,&item.UserID,&item.DeviceID,&item.SiteID,&item.Name,&item.Command,&item.IntervalMinutes)==nil{items=append(items,item)}}
	rows.Close()
	for _,item:=range items{s.queueScheduledTask(item)}
}

func (s *Server) queueScheduledTask(item dueScheduledTask){
	ctx,cancel:=context.WithTimeout(context.Background(),10*time.Second);defer cancel()
	res,err:=s.db.conn.ExecContext(ctx,`UPDATE scheduled_tasks SET next_run_at=DATE_ADD(UTC_TIMESTAMP(),INTERVAL interval_minutes MINUTE),last_run_at=UTC_TIMESTAMP(),last_status='queued',last_output=NULL,last_error=NULL WHERE id=? AND enabled=TRUE AND next_run_at<=UTC_TIMESTAMP()`,item.ID)
	if err!=nil{return};n,_:=res.RowsAffected();if n!=1{return}
	conn,online:=s.hub.Get(item.DeviceID)
	if !online{
		_,_ = s.db.conn.ExecContext(ctx,`UPDATE scheduled_tasks SET next_run_at=DATE_ADD(UTC_TIMESTAMP(),INTERVAL 15 MINUTE),last_status='failed',last_error='device offline; retry in 15 minutes' WHERE id=?`,item.ID)
		_,_ = s.db.conn.ExecContext(ctx,`INSERT INTO notifications(user_id,type,title,body) VALUES(?,?,?,?)`,item.UserID,"error","Scheduled task delayed: "+item.Name,"The device is offline. Yevqo will retry in 15 minutes.")
		return
	}
	go s.executeScheduledTask(item,conn)
}

func (s *Server) executeScheduledTask(item dueScheduledTask,conn *AgentConn){
	ctx,cancel:=context.WithTimeout(context.Background(),35*time.Minute);defer cancel()
	result,err:=conn.SendCommand(ctx,"run_scheduled_task",&shared.RunScheduledTask{SiteId:item.SiteID,TaskID:item.ID,Command:item.Command,TimeoutSeconds:1800})
	status,output,reason:="completed","",""
	if err!=nil{status="failed";reason=err.Error()}else if result==nil||!result.Ok{status="failed";if result!=nil{reason=result.Message}}else{
		var payload shared.ScheduledTaskResult
		if err:=shared.Unmarshal(result.Payload,&payload);err!=nil{status="failed";reason="invalid Agent task result"}else{output=payload.Output;if payload.ExitCode!=0{status="failed";reason=fmt.Sprintf("command exited with code %d",payload.ExitCode)}}
	}
	if len(output)>60000{output=output[:60000]}
	_,_ = s.db.conn.ExecContext(context.Background(),`UPDATE scheduled_tasks SET last_status=?,last_output=?,last_error=? WHERE id=?`,status,output,nullableString(reason),item.ID)
	notificationType:="success";title:="Scheduled task completed: "+item.Name;body:=fmt.Sprintf("Task for %s completed successfully.",item.SiteID)
	if status=="failed"{notificationType="error";title="Scheduled task failed: "+item.Name;body=reason;if body==""{body="The scheduled command failed."}}
	_,_ = s.db.conn.ExecContext(context.Background(),`INSERT INTO notifications(user_id,type,title,body) VALUES(?,?,?,?)`,item.UserID,notificationType,title,truncateNotification(body))
	_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(item.UserID),DeviceID:shared.PtrInt64(item.DeviceID),Action:"scheduled_task_"+status,Details:map[string]interface{}{"task_id":item.ID,"site_id":item.SiteID,"output_bytes":len(output)}})
}
