package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

type linuxProcessCounters struct {
	cpu uint64
	rss uint64
	count int
}

// parseLinuxProcessStat reads fields after the final ')' because Linux comm
// values may themselves contain spaces and parentheses.
func parseLinuxProcessStat(line string) (pid, processGroup int, counters linuxProcessCounters, err error) {
	end := strings.LastIndex(line, ")")
	if end < 0 || end+1 >= len(line) {
		return 0, 0, counters, fmt.Errorf("invalid process stat")
	}
	fields := strings.Fields(line[end+1:])
	if len(fields) < 22 {
		return 0, 0, counters, fmt.Errorf("incomplete process stat")
	}
	pidField := strings.Fields(line[:strings.Index(line, " ")])
	if len(pidField) != 1 {
		return 0, 0, counters, fmt.Errorf("invalid process id")
	}
	pid, err = strconv.Atoi(pidField[0])
	if err != nil || pid < 1 {
		return 0, 0, counters, fmt.Errorf("invalid process id")
	}
	processGroup, err = strconv.Atoi(fields[2])
	if err != nil || processGroup < 1 {
		return 0, 0, counters, fmt.Errorf("invalid process group")
	}
	utime, err := strconv.ParseUint(fields[11], 10, 64)
	if err != nil {
		return 0, 0, counters, err
	}
	stime, err := strconv.ParseUint(fields[12], 10, 64)
	if err != nil {
		return 0, 0, counters, err
	}
	rssPages, err := strconv.ParseInt(fields[21], 10, 64)
	if err != nil || rssPages < 0 {
		return 0, 0, counters, fmt.Errorf("invalid process RSS")
	}
	return pid, processGroup, linuxProcessCounters{
		cpu: utime + stime,
		rss: uint64(rssPages) * uint64(os.Getpagesize()),
		count: 1,
	}, nil
}

// readLinuxProcessGroupCountersAt aggregates every process in a dedicated
// process group. Each managed site owns its own process group, so this also
// includes npm/node workers and other descendants without trusting parent PIDs.
func readLinuxProcessGroupCountersAt(procRoot string, processGroup int) (linuxProcessCounters, error) {
	if processGroup < 1 {
		return linuxProcessCounters{}, fmt.Errorf("invalid process group")
	}
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return linuxProcessCounters{}, err
	}
	var total linuxProcessCounters
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(procRoot, entry.Name(), "stat"))
		if readErr != nil {
			// Processes may exit while /proc is being enumerated.
			continue
		}
		_, group, counters, parseErr := parseLinuxProcessStat(string(data))
		if parseErr != nil || group != processGroup {
			continue
		}
		total.cpu += counters.cpu
		total.rss += counters.rss
		total.count++
	}
	if total.count == 0 {
		return linuxProcessCounters{}, fmt.Errorf("process group %d is not present", processGroup)
	}
	return total, nil
}

func readLinuxProcessGroupCounters(processGroup int) (linuxProcessCounters, error) {
	return readLinuxProcessGroupCountersAt("/proc", processGroup)
}

func readLinuxProcessCounters(pid int) (linuxProcessCounters, error) {
	if pid < 1 {
		return linuxProcessCounters{}, fmt.Errorf("invalid process id")
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return linuxProcessCounters{}, err
	}
	_, _, counters, err := parseLinuxProcessStat(string(data))
	return counters, err
}

func readLinuxTotalCPUTicks() (uint64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)[1:]
		var total uint64
		for _, field := range fields {
			value, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				return 0, err
			}
			total += value
		}
		return total, nil
	}
	return 0, fmt.Errorf("system CPU counters are unavailable")
}

func (m *ProcessManager) GetSiteMetrics(siteID string) (*shared.SiteMetricsResult, error) {
	if !validSiteID(siteID) {
		return nil, fmt.Errorf("invalid site id")
	}
	if m.registry == nil {
		return nil, fmt.Errorf("process registry is unavailable")
	}
	record, ok := m.registry.Get(siteID)
	if !ok || record.PID < 1 {
		return nil, fmt.Errorf("site process is not managed")
	}
	if !verifyProcessRecord(record) {
		return nil, fmt.Errorf("site process identity could not be verified")
	}
	group := record.ProcessGroup
	if group < 1 {
		// Older records always used the leader PID as their process-group ID.
		group = record.PID
	}
	first, err := readLinuxProcessGroupCounters(group)
	if err != nil {
		return nil, fmt.Errorf("read site process-group metrics: %w", err)
	}
	totalFirst, totalErr := readLinuxTotalCPUTicks()
	time.Sleep(200 * time.Millisecond)
	second, err := readLinuxProcessGroupCounters(group)
	if err != nil {
		return nil, fmt.Errorf("site process group exited while collecting metrics: %w", err)
	}
	totalSecond, totalErr2 := readLinuxTotalCPUTicks()
	cpu := 0.0
	if totalErr == nil && totalErr2 == nil && totalSecond > totalFirst && second.cpu >= first.cpu {
		cpu = float64(second.cpu-first.cpu) / float64(totalSecond-totalFirst) * float64(runtime.NumCPU()) * 100
		if cpu < 0 {
			cpu = 0
		}
	}
	return &shared.SiteMetricsResult{
		SiteId: siteID,
		PID: record.PID,
		ProcessCount: second.count,
		CPUPercent: cpu,
		RSSBytes: second.rss,
		CapturedAt: time.Now().UTC(),
	}, nil
}
