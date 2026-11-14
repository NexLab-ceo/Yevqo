package control

import (
	"context"
	"fmt"
	"time"
)

type DeviceMetricPoint struct {
	CPUPercent        float64   `json:"cpu_percent"`
	MemoryUsedPercent float64   `json:"memory_used_percent"`
	CapturedAt        time.Time `json:"captured_at"`
}

func (db *DB) ListDeviceMetricHistory(ctx context.Context, deviceID int64, limit int) ([]DeviceMetricPoint, error) {
	if limit < 1 || limit > 120 {
		limit = 20
	}
	rows, err := db.conn.QueryContext(ctx, fmt.Sprintf(`
		SELECT cpu_percent,
			CASE WHEN memory_total_bytes > 0
				THEN 100.0 * (memory_total_bytes - memory_available_bytes) / memory_total_bytes
				ELSE 0 END,
			captured_at
		FROM device_metrics
		WHERE device_id = ?
		ORDER BY captured_at DESC
		LIMIT %d
	`, limit), deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	points := make([]DeviceMetricPoint, 0, limit)
	for rows.Next() {
		var p DeviceMetricPoint
		if err := rows.Scan(&p.CPUPercent, &p.MemoryUsedPercent, &p.CapturedAt); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}
	return points, nil
}
