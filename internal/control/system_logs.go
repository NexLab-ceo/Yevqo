package control

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const systemLogRetentionDays = 30

var systemLogPruneCounter uint64

type systemRequestIDContextKey struct{}

func systemRequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(systemRequestIDContextKey{}).(string)
	return value
}

type systemLogRecord struct {
	Context context.Context
	Level string
	Category string
	Source string
	Message string
	RequestID string
	UserID int64
	DeviceID int64
	Method string
	Path string
	Status int
	Duration time.Duration
	Metadata map[string]interface{}
}

func (s *Server) recordSystemLog(entry systemLogRecord) {
	if s == nil || s.db == nil || s.db.conn == nil { return }
	if entry.RequestID == "" && entry.Context != nil { entry.RequestID = systemRequestIDFromContext(entry.Context) }
	if entry.Level == "" { entry.Level = "info" }
	if entry.Category == "" { entry.Category = "system" }
	if entry.Source == "" { entry.Source = "control-plane" }
	if len(entry.Message) > 512 { entry.Message = entry.Message[:512] }
	if len(entry.Path) > 512 { entry.Path = entry.Path[:512] }
	if len(entry.Metadata) == 0 { entry.Metadata = nil }
	metadata, err := json.Marshal(entry.Metadata)
	if err != nil { metadata = []byte("{}") }
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	_, err = s.db.conn.ExecContext(ctx, `INSERT INTO system_logs
		(level,category,source,message,request_id,user_id,device_id,method,path,status_code,duration_ms,metadata)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		entry.Level, entry.Category, entry.Source, entry.Message,
		nullableString(entry.RequestID), nullableSystemLogInt(entry.UserID), nullableSystemLogInt(entry.DeviceID),
		nullableString(entry.Method), nullableString(entry.Path),
		nullableSystemLogInt(int64(entry.Status)), nullableSystemLogInt(entry.Duration.Milliseconds()), metadata)
	if err != nil {
		// Keep the file log as a fallback; never recursively log a DB logging failure.
		log.Printf("system log persistence failed: %v", err)
		return
	}
	// Prune in small, infrequent batches so normal log writes stay cheap.
	// The created_at index bounds the scan to expired rows.
	if atomic.AddUint64(&systemLogPruneCounter, 1)%256 == 0 {
		pruneCtx, pruneCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer pruneCancel()
		if _, pruneErr := s.db.conn.ExecContext(pruneCtx,
			"DELETE FROM system_logs WHERE created_at < DATE_SUB(NOW(), INTERVAL 30 DAY)"); pruneErr != nil {
			log.Printf("system log retention cleanup failed: %v", pruneErr)
		}
	}
}

func nullableSystemLogInt(value int64) interface{} { if value <= 0 { return nil }; return value }

func newSystemRequestID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil { return strconv.FormatInt(time.Now().UnixNano(), 36) }
	return hex.EncodeToString(raw[:])
}

func (s *Server) handleSystemLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { http.Error(w, "method not allowed", http.StatusMethodNotAllowed); return }
	limit := 200
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 500 { apiError(w, fmt.Errorf("limit must be between 1 and 500")); return }
		limit = n
	}
	level := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("level")))
	category := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("category")))
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	if level != "" && level != "debug" && level != "info" && level != "warn" && level != "error" { http.Error(w, "invalid level", http.StatusBadRequest); return }
	if len(category) > 48 || len(search) > 160 { http.Error(w, "filter is too long", http.StatusBadRequest); return }
	where := []string{"1=1"}
	args := []interface{}{}
	if level != "" { where = append(where, "level=?"); args = append(args, level) }
	if category != "" { where = append(where, "category=?"); args = append(args, category) }
	if search != "" { where = append(where, "(message LIKE ? OR source LIKE ? OR path LIKE ? OR request_id LIKE ?)"); term := "%" + search + "%"; args = append(args, term, term, term, term) }
	filter := strings.Join(where, " AND ")
	var total, errorsCount, warningsCount int64
	_ = s.db.conn.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM system_logs WHERE "+filter, args...).Scan(&total)
	_ = s.db.conn.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM system_logs WHERE level='error'").Scan(&errorsCount)
	_ = s.db.conn.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM system_logs WHERE level='warn'").Scan(&warningsCount)
	query := "SELECT id,level,category,source,message,request_id,user_id,device_id,method,path,status_code,duration_ms,metadata,created_at FROM system_logs WHERE "+filter+" ORDER BY id DESC LIMIT ?"
	queryArgs := append(append([]interface{}{}, args...), limit)
	rows, err := s.db.conn.QueryContext(r.Context(), query, queryArgs...)
	if err != nil { apiError(w, fmt.Errorf("could not query system logs: %w", err)); return }
	defer rows.Close()
	out := make([]map[string]interface{}, 0, limit)
	for rows.Next() {
		var id int64
		var lv, cat, source, message string
		var requestID, method, path sql.NullString
		var userID, deviceID, status, duration sql.NullInt64
		var metadata []byte
		var created time.Time
		if err := rows.Scan(&id,&lv,&cat,&source,&message,&requestID,&userID,&deviceID,&method,&path,&status,&duration,&metadata,&created); err != nil { apiError(w, fmt.Errorf("could not read system log row: %w", err)); return }
		var details interface{}
		if len(metadata)>0 { _ = json.Unmarshal(metadata,&details) }
		out = append(out,map[string]interface{}{"id":id,"level":lv,"category":cat,"source":source,"message":message,"request_id":requestID.String,"user_id":userID.Int64,"device_id":deviceID.Int64,"method":method.String,"path":path.String,"status_code":status.Int64,"duration_ms":duration.Int64,"metadata":details,"created_at":created})
	}
	if err := rows.Err(); err != nil { apiError(w, fmt.Errorf("could not finish reading system logs: %w", err)); return }
	writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"logs":out,"summary":map[string]interface{}{"total":total,"errors":errorsCount,"warnings":warningsCount}})
}
