package agent

import (
 "bufio"
 "fmt"
 "net"
 "os"
 "path/filepath"
 "os/exec"
 "strconv"
 "strings"
 "time"
)

type packageInfo struct { Installed bool `json:"installed"`; Version string `json:"version,omitempty"` }
type deviceDiagnostics struct {
 LocalIP string `json:"local_ip,omitempty"`
 BatteryPercent *int `json:"battery_percent,omitempty"`
 BatteryStatus string `json:"battery_status,omitempty"`
 DeviceUptimeSeconds int64 `json:"device_uptime_seconds,omitempty"`
 AgentUptimeSeconds int64 `json:"agent_uptime_seconds,omitempty"`
 ListeningPorts []int `json:"listening_ports"`
 Packages map[string]packageInfo `json:"packages"`
 AgentLogs string `json:"agent_logs,omitempty"`
 CapturedAt time.Time `json:"captured_at"`
}
var agentStartedAt = time.Now()

// OpenAgentLog creates a private, bounded Agent log file without following symlinks.
func OpenAgentLog(dataRoot string) (*os.File,error) {
 dir:=filepath.Join(dataRoot,"logs")
 if err:=os.MkdirAll(dir,0700);err!=nil{return nil,err}
 info,err:=os.Lstat(dir);if err!=nil{return nil,err};if !info.IsDir()||info.Mode()&os.ModeSymlink!=0{return nil,fmt.Errorf("Agent log directory is not a safe directory")}
 if err:=os.Chmod(dir,0700);err!=nil{return nil,err}
 path:=filepath.Join(dir,"agent.log")
 if info,err:=os.Lstat(path);err==nil {
  if !info.Mode().IsRegular()||info.Mode()&os.ModeSymlink!=0{return nil,fmt.Errorf("Agent log is not a regular file")}
  if info.Size()>5<<20 {
   rotated:=path+".1";if old,e:=os.Lstat(rotated);e==nil {if !old.Mode().IsRegular()||old.Mode()&os.ModeSymlink!=0{return nil,fmt.Errorf("rotated Agent log is not a regular file")};if e=os.Remove(rotated);e!=nil{return nil,e}}
   if err:=os.Rename(path,rotated);err!=nil{return nil,err}
   if err:=os.Chmod(rotated,0600);err!=nil{return nil,err}
  }
 } else if !os.IsNotExist(err){return nil,err}
 if err:=os.Chmod(path,0600);err!=nil&&!os.IsNotExist(err){return nil,err}
 return os.OpenFile(path,os.O_CREATE|os.O_APPEND|os.O_WRONLY,0600)
}

func commandVersion(binary string, args ...string) packageInfo {
 path, err := exec.LookPath(binary)
 if err != nil { return packageInfo{Installed:false} }
 out, err := exec.Command(path,args...).CombinedOutput()
 version := strings.TrimSpace(string(out))
 if len(version)>240 { version=version[:240] }
 if err != nil && version=="" { version="installed (version unavailable)" }
 if version=="" { version="installed" }
 return packageInfo{Installed:true,Version:version}
}

func readBattery() (*int,string) {
 for _, root := range []string{"/sys/class/power_supply/battery","/sys/class/power_supply/BAT0","/sys/class/power_supply/battery/"} {
  b,err:=os.ReadFile(root+"/capacity")
  if err==nil { v,e:=strconv.Atoi(strings.TrimSpace(string(b))); if e==nil&&v>=0&&v<=100 {
   statusBytes,_:=os.ReadFile(root+"/status")
   status:=strings.TrimSpace(string(statusBytes))
   if status=="" {status="unknown"}
   return &v,status
  }}
 }
 return nil,""
}

func readDeviceUptime() int64 {
 b,err:=os.ReadFile("/proc/uptime");if err!=nil{return 0}
 fields:=strings.Fields(string(b));if len(fields)==0{return 0}
 seconds,err:=strconv.ParseFloat(fields[0],64);if err!=nil||seconds<0{return 0}
 return int64(seconds)
}

func readListeningPorts() []int {
 ports:=map[int]bool{}
 for _,path:=range []string{"/proc/net/tcp","/proc/net/tcp6"} {
  f,err:=os.Open(path);if err!=nil{continue}
  scanner:=bufio.NewScanner(f)
  if scanner.Scan(){ /* header */ }
  for scanner.Scan(){
   fields:=strings.Fields(scanner.Text())
   if len(fields)<4||fields[3]!="0A"{continue}
   local:=strings.Split(fields[1],":");if len(local)!=2{continue}
   p,err:=strconv.ParseInt(local[1],16,32);if err==nil&&p>0&&p<=65535{ports[int(p)]=true}
  }
  _=f.Close()
 }
 out:=make([]int,0,len(ports));for p:=range ports{out=append(out,p)}
 // Keep output bounded and deterministic.
 for i:=0;i<len(out);i++{for j:=i+1;j<len(out);j++{if out[j]<out[i]{out[i],out[j]=out[j],out[i]}}}
 if len(out)>256{out=out[:256]}
 return out
}

func primaryLocalIP() string {
 addrs,err:=net.InterfaceAddrs();if err!=nil{return ""}
 for _,addr:=range addrs { ipnet,ok:=addr.(*net.IPNet);if !ok||ipnet.IP.IsLoopback(){continue};if ip4:=ipnet.IP.To4();ip4!=nil&&ip4.IsPrivate(){return ip4.String()} }
 return ""
}

func readAgentLogs(dataRoot string) string {
 logPath:=filepath.Join(dataRoot,"logs","agent.log")
 if info,err:=os.Lstat(logPath);err==nil&&info.Mode().IsRegular()&&info.Mode()&os.ModeSymlink==0 {
  data,readErr:=os.ReadFile(logPath);if readErr==nil { lines:=strings.Split(strings.TrimRight(string(data),"\n"),"\n");if len(lines)>200{lines=lines[len(lines)-200:]};return strings.Join(lines,"\n") }
 }

 prefix:=os.Getenv("PREFIX");if prefix==""{prefix="/data/data/com.termux/files/usr"}
 service:="yevqo";if _,err:=os.Stat(prefix+"/var/service/yevqo");err!=nil{service="yevqo-agent"}
 binary,err:=exec.LookPath("svlogtail");if err!=nil{return "svlogtail недоступен в PATH"}
 out,err:=exec.Command(binary,service).CombinedOutput();text:=strings.TrimSpace(string(out));if len(text)>24000{text=text[len(text)-24000:]};if err!=nil&&text==""{return "Логи сервиса пока недоступны"};return text
}

func collectDeviceDiagnostics(dataRoot string) deviceDiagnostics {
 batteryPercent,batteryStatus:=readBattery()
 return deviceDiagnostics{
  LocalIP:primaryLocalIP(),
  BatteryPercent:batteryPercent,
  BatteryStatus:batteryStatus,
  DeviceUptimeSeconds:readDeviceUptime(),
  AgentUptimeSeconds:int64(time.Since(agentStartedAt).Seconds()),
  ListeningPorts:readListeningPorts(),
  Packages:map[string]packageInfo{
   "php":commandVersion("php","-v"),
   "node":commandVersion("node","--version"),
   "git":commandVersion("git","--version"),
   "cloudflared":commandVersion("cloudflared","--version"),
  },
  AgentLogs:readAgentLogs(dataRoot),
  CapturedAt:time.Now().UTC(),
 }
}

func checkLocalPort(port int) (bool,error) {
 if port<1||port>65535{return false,fmt.Errorf("port must be between 1 and 65535")}
 conn,err:=net.DialTimeout("tcp",net.JoinHostPort("127.0.0.1",strconv.Itoa(port)),1200*time.Millisecond)
 if err!=nil{return false,nil}
 _=conn.Close()
 return true,nil
}
