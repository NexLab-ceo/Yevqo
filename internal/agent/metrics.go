package agent

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"veltrix/internal/shared"
)

func cpuCounters() (idle, total uint64, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil { return 0, 0, err }
	defer f.Close()
	scanner := bufio.NewScanner(f)
	if !scanner.Scan() { return 0, 0, fmt.Errorf("cpu stats unavailable") }
	fields := strings.Fields(scanner.Text())
	if len(fields) < 5 || fields[0] != "cpu" { return 0, 0, fmt.Errorf("invalid cpu stats") }
	for i := 1; i < len(fields); i++ {
		v, e := strconv.ParseUint(fields[i], 10, 64)
		if e != nil { continue }
		total += v
		if i == 4 || i == 5 { idle += v }
	}
	return idle, total, nil
}

func collectMetrics(dataRoot string) (*shared.MetricsResult, error) {
	m := &shared.MetricsResult{CapturedAt: time.Now().UTC()}
	idle1, total1, err := cpuCounters()
	if err == nil {
		time.Sleep(250 * time.Millisecond)
		idle2, total2, e := cpuCounters()
		if e == nil && total2 > total1 {
			m.CPUPercent = 100 * float64((total2-total1)-(idle2-idle1)) / float64(total2-total1)
		}
	}
	if b, e := os.ReadFile("/proc/loadavg"); e == nil {
		fields := strings.Fields(string(b))
		if len(fields) > 0 { m.Load1m, _ = strconv.ParseFloat(fields[0], 64) }
	}
	mem := map[string]uint64{}
	if f, e := os.Open("/proc/meminfo"); e == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 2 {
				v, parseErr := strconv.ParseUint(fields[1], 10, 64)
				if parseErr == nil { mem[strings.TrimSuffix(fields[0], ":")] = v * 1024 }
			}
		}
		_ = f.Close()
	}
	m.MemoryTotalBytes = mem["MemTotal"]
	m.MemoryAvailableBytes = mem["MemAvailable"]
	if m.MemoryAvailableBytes == 0 { m.MemoryAvailableBytes = mem["MemFree"] }
	var stat syscall.Statfs_t
	if e := syscall.Statfs(dataRoot, &stat); e == nil {
		m.StorageTotalBytes = stat.Blocks * uint64(stat.Bsize)
		m.StorageAvailableBytes = stat.Bavail * uint64(stat.Bsize)
	}
	paths, _ := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	for _, path := range paths {
		if b, e := os.ReadFile(path); e == nil {
			v, parseErr := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
			if parseErr == nil {
				if v > 1000 { v /= 1000 }
				if v > 0 && v < 150 { m.TemperatureCelsius = &v; break }
			}
		}
	}
	if m.MemoryTotalBytes == 0 && m.StorageTotalBytes == 0 { return nil, fmt.Errorf("device metrics are unavailable") }
	return m, nil
}
