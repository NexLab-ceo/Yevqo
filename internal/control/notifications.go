package control

import (
	"context"
	"fmt"
	"encoding/json"
	"strings"

	"veltrix/internal/shared"
)


// notificationPreferenceEnabled checks the recipient's persisted notification category.
func (s *Server) notificationPreferenceEnabled(ctx context.Context, userID int64, category string) bool {
	var raw []byte
	if err := s.db.conn.QueryRowContext(ctx, `SELECT setting_value FROM user_settings WHERE user_id=? AND setting_key='notifications'`, userID).Scan(&raw); err != nil { return true }
	var preferences map[string]bool
	if json.Unmarshal(raw, &preferences) != nil { return true }
	enabled, exists := preferences[category]
	return !exists || enabled
}

func notificationCategory(kind, title string) string {
	lower := strings.ToLower(title)
	if strings.Contains(lower, "deployment") || strings.Contains(lower, "deploy") || strings.Contains(lower, "build") {
		if kind == "error" { return "deployment_errors" }
		return "system"
	}
	if strings.Contains(lower, "device") || strings.Contains(lower, "agent") { return "device_events" }
	if strings.Contains(lower, "project") || strings.Contains(lower, "workspace") { return "project_updates" }
	return "system"
}

// notifyWorkspaceEvent delivers a resource event to its workspace owner and,
// when different, to the member who initiated the action.
func (s *Server) notifyWorkspaceEvent(ctx context.Context, ownerID, actorID int64, kind, title, body string) {
	recipients := []int64{ownerID}
	if actorID > 0 && actorID != ownerID {
		recipients = append(recipients, actorID)
	}
	for _, recipientID := range recipients {
		if !s.notificationPreferenceEnabled(ctx, recipientID, notificationCategory(kind,title)) { continue }
		_, _ = s.db.conn.ExecContext(ctx, `INSERT INTO notifications(user_id,type,title,body) VALUES(?,?,?,?)`, recipientID, kind, title, body)
	}
}

type metricAlert struct {
	key string
	label string
	value float64
	threshold float64
	recovery float64
	unit string
}

func metricAlertTransition(active bool, value, threshold, recovery float64) (bool, string) {
	if !active && value >= threshold { return true, "triggered" }
	if active && value <= recovery { return false, "recovered" }
	return active, ""
}

func (s *Server) evaluateDeviceMetricAlerts(ctx context.Context, userID, deviceID int64, metrics shared.MetricsResult) error {
	if userID < 1 || deviceID < 1 { return nil }
	var hostname string
	if err := s.db.conn.QueryRowContext(ctx, `SELECT hostname FROM devices WHERE id=? AND user_id=?`, deviceID, userID).Scan(&hostname); err != nil { return err }
	if hostname == "" { hostname = fmt.Sprintf("Device #%d", deviceID) }
	alerts := make([]metricAlert, 0, 4)
	if metrics.CPUPercent >= 0 && metrics.CPUPercent <= 100 {
		alerts = append(alerts, metricAlert{key:"cpu",label:"CPU usage",value:metrics.CPUPercent,threshold:90,recovery:85,unit:"%"})
	}
	if metrics.MemoryTotalBytes > 0 {
		available := metrics.MemoryAvailableBytes
		if available > metrics.MemoryTotalBytes { available = metrics.MemoryTotalBytes }
		used := 100 * float64(metrics.MemoryTotalBytes-available) / float64(metrics.MemoryTotalBytes)
		alerts = append(alerts, metricAlert{key:"memory",label:"Memory usage",value:used,threshold:90,recovery:85,unit:"%"})
	}
	if metrics.StorageTotalBytes > 0 {
		available := metrics.StorageAvailableBytes
		if available > metrics.StorageTotalBytes { available = metrics.StorageTotalBytes }
		used := 100 * float64(metrics.StorageTotalBytes-available) / float64(metrics.StorageTotalBytes)
		alerts = append(alerts, metricAlert{key:"storage",label:"Storage usage",value:used,threshold:90,recovery:85,unit:"%"})
	}
	if metrics.TemperatureCelsius != nil && *metrics.TemperatureCelsius >= -40 && *metrics.TemperatureCelsius <= 150 {
		alerts = append(alerts, metricAlert{key:"temperature",label:"Device temperature",value:*metrics.TemperatureCelsius,threshold:80,recovery:75,unit:"°C"})
	}
	if len(alerts) == 0 { return nil }
	tx, err := s.db.conn.BeginTx(ctx, nil)
	if err != nil { return err }
	defer tx.Rollback()
	for _, alert := range alerts {
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO device_alert_state(device_id,user_id,alert_key,active,last_value) VALUES(?,?,?,0,?)`, deviceID,userID,alert.key,alert.value); err != nil { return err }
		var active bool
		if err := tx.QueryRowContext(ctx, `SELECT active FROM device_alert_state WHERE device_id=? AND alert_key=? FOR UPDATE`,deviceID,alert.key).Scan(&active); err != nil { return err }
		nextActive, transition := metricAlertTransition(active,alert.value,alert.threshold,alert.recovery)
		if _, err := tx.ExecContext(ctx, `UPDATE device_alert_state SET user_id=?,active=?,last_value=?,updated_at=CURRENT_TIMESTAMP WHERE device_id=? AND alert_key=?`,userID,nextActive,alert.value,deviceID,alert.key); err != nil { return err }
		if transition == "" { continue }
		title := fmt.Sprintf("%s on %s", alert.label, hostname)
		kind := "warning"
		if transition == "recovered" { title = fmt.Sprintf("%s recovered on %s", alert.label, hostname); kind = "success" }
		body := fmt.Sprintf("%s is %.1f%s; alert threshold %.1f%s, recovery threshold %.1f%s.",alert.label,alert.value,alert.unit,alert.threshold,alert.unit,alert.recovery,alert.unit)
		if s.notificationPreferenceEnabled(ctx,userID,"device_events") { if _, err := tx.ExecContext(ctx, `INSERT INTO notifications(user_id,type,title,body) VALUES(?,?,?,?)`,userID,kind,title,body); err != nil { return err } }
	}
	return tx.Commit()
}

func (s *Server) evaluateDeviceConnectivityAlert(ctx context.Context, userID, deviceID int64, online bool) error {
	if userID < 1 || deviceID < 1 { return nil }
	var hostname string
	if err := s.db.conn.QueryRowContext(ctx, `SELECT hostname FROM devices WHERE id=? AND user_id=?`,deviceID,userID).Scan(&hostname); err != nil { return err }
	if hostname == "" { hostname = fmt.Sprintf("Device #%d",deviceID) }
	value := 1.0
	if online { value = 0 }
	tx, err := s.db.conn.BeginTx(ctx,nil)
	if err != nil { return err }
	defer tx.Rollback()
	if _,err:=tx.ExecContext(ctx,`INSERT IGNORE INTO device_alert_state(device_id,user_id,alert_key,active,last_value) VALUES(?,?, 'offline',0,?)`,deviceID,userID,value);err!=nil{return err}
	var active bool
	if err:=tx.QueryRowContext(ctx,`SELECT active FROM device_alert_state WHERE device_id=? AND alert_key='offline' FOR UPDATE`,deviceID).Scan(&active);err!=nil{return err}
	nextActive,transition:=metricAlertTransition(active,value,1,0)
	if _,err:=tx.ExecContext(ctx,`UPDATE device_alert_state SET user_id=?,active=?,last_value=?,updated_at=CURRENT_TIMESTAMP WHERE device_id=? AND alert_key='offline'`,userID,nextActive,value,deviceID);err!=nil{return err}
	if transition!=""{
		kind,title,body:="warning","Device disconnected: "+hostname,"The Agent connection to Yevqo was lost."
		if transition=="recovered"{kind="success";title="Device reconnected: "+hostname;body="The Agent has reconnected to Yevqo."}
		if s.notificationPreferenceEnabled(ctx,userID,"device_events"){if _,err:=tx.ExecContext(ctx,`INSERT INTO notifications(user_id,type,title,body) VALUES(?,?,?,?)`,userID,kind,title,body);err!=nil{return err}}
	}
	return tx.Commit()
}
