package agent

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

type linuxProcessCounters struct { cpu uint64; rss uint64 }

func readLinuxProcessCounters(pid int) (linuxProcessCounters,error) {
	if pid<1{return linuxProcessCounters{},fmt.Errorf("invalid process id")}
	data,err:=os.ReadFile(fmt.Sprintf("/proc/%d/stat",pid));if err!=nil{return linuxProcessCounters{},err}
	line:=string(data);end:=strings.LastIndex(line,")")
	if end<0||end+1>=len(line){return linuxProcessCounters{},fmt.Errorf("invalid process stat")}
	fields:=strings.Fields(line[end+1:])
	if len(fields)<22{return linuxProcessCounters{},fmt.Errorf("incomplete process stat")}
	utime,err:=strconv.ParseUint(fields[11],10,64);if err!=nil{return linuxProcessCounters{},err}
	stime,err:=strconv.ParseUint(fields[12],10,64);if err!=nil{return linuxProcessCounters{},err}
	rssPages,err:=strconv.ParseInt(fields[21],10,64);if err!=nil||rssPages<0{return linuxProcessCounters{},fmt.Errorf("invalid process RSS")}
	return linuxProcessCounters{cpu:utime+stime,rss:uint64(rssPages)*uint64(os.Getpagesize())},nil
}

func readLinuxTotalCPUTicks()(uint64,error){
	data,err:=os.ReadFile("/proc/stat");if err!=nil{return 0,err}
	for _,line:=range strings.Split(string(data),"\n"){
		if !strings.HasPrefix(line,"cpu "){continue}
		fields:=strings.Fields(line)[1:];var total uint64
		for _,field:=range fields{value,err:=strconv.ParseUint(field,10,64);if err!=nil{return 0,err};total+=value}
		return total,nil
	}
	return 0,fmt.Errorf("system CPU counters are unavailable")
}

func (m *ProcessManager) GetSiteMetrics(siteID string)(*shared.SiteMetricsResult,error){
	if !validSiteID(siteID){return nil,fmt.Errorf("invalid site id")}
	if m.registry==nil{return nil,fmt.Errorf("process registry is unavailable")}
	record,ok:=m.registry.Get(siteID);if !ok||record.PID<1{return nil,fmt.Errorf("site process is not managed")}
	if !verifyProcessRecord(record){return nil,fmt.Errorf("site process identity could not be verified")}
	first,err:=readLinuxProcessCounters(record.PID);if err!=nil{return nil,fmt.Errorf("read process metrics: %w",err)}
	totalFirst,totalErr:=readLinuxTotalCPUTicks()
	time.Sleep(200*time.Millisecond)
	second,err:=readLinuxProcessCounters(record.PID);if err!=nil{return nil,fmt.Errorf("process exited while collecting metrics: %w",err)}
	totalSecond,totalErr2:=readLinuxTotalCPUTicks()
	cpu:=0.0
	if totalErr==nil&&totalErr2==nil&&totalSecond>totalFirst&&second.cpu>=first.cpu{
		cpu=float64(second.cpu-first.cpu)/float64(totalSecond-totalFirst)*float64(runtime.NumCPU())*100
		if cpu<0{cpu=0}
	}
	return &shared.SiteMetricsResult{SiteId:siteID,PID:record.PID,CPUPercent:cpu,RSSBytes:second.rss,CapturedAt:time.Now().UTC()},nil
}
