package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"veltrix/internal/shared"
)

type cappedTaskOutput struct { mu sync.Mutex; data []byte; limit int }
func (b *cappedTaskOutput) Write(p []byte)(int,error){
	b.mu.Lock();defer b.mu.Unlock()
	n:=len(p);remaining:=b.limit-len(b.data)
	if remaining>0{if len(p)>remaining{p=p[:remaining]};b.data=append(b.data,p...)}
	return n,nil
}
func (b *cappedTaskOutput) String()string{b.mu.Lock();defer b.mu.Unlock();return string(b.data)}

func (m *ProcessManager) RunScheduledTask(req shared.RunScheduledTask)(*shared.ScheduledTaskResult,error){
	if !validSiteID(req.SiteId)||req.TaskID<1||strings.TrimSpace(req.Command)==""||len(req.Command)>4096||strings.IndexByte(req.Command,0)>=0{return nil,fmt.Errorf("invalid scheduled task")}
	if req.TimeoutSeconds<30||req.TimeoutSeconds>1800{return nil,fmt.Errorf("task timeout must be between 30 and 1800 seconds")}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	siteRoot,err:=safeExistingSubdirectory(m.DataRoot,filepath.Join("sites",req.SiteId));if err!=nil{return nil,fmt.Errorf("site directory unavailable: %w",err)}
	envValues,err:=m.loadSiteEnvironment(req.SiteId);if err!=nil{return nil,fmt.Errorf("could not load protected site environment: %w",err)}
	env:=append([]string{},os.Environ()...)
	for key,value:=range envValues{
		if !validManagedEnvironmentKey(key)||len(value)>16384||strings.IndexByte(value,0)>=0{return nil,fmt.Errorf("invalid protected environment value")}
		env=append(env,key+"="+value)
	}
	ctx,cancel:=context.WithTimeout(context.Background(),time.Duration(req.TimeoutSeconds)*time.Second);defer cancel()
	cmd:=exec.CommandContext(ctx,"sh","-lc",req.Command)
	cmd.Dir=siteRoot
	cmd.Env=env
	cmd.SysProcAttr=&syscall.SysProcAttr{Setpgid:true}
	output:=&cappedTaskOutput{limit:256*1024}
	cmd.Stdout=output;cmd.Stderr=output
	started:=time.Now()
	err=cmd.Run()
	if ctx.Err()!=nil&&cmd.Process!=nil{_ = killProcessGroup(cmd.Process.Pid,cmd.Process.Pid)}
	result:=&shared.ScheduledTaskResult{ExitCode:0,Output:output.String(),DurationMS:time.Since(started).Milliseconds()}
	if err!=nil{
		result.ExitCode=1
		var exitErr *exec.ExitError
		if errors.As(err,&exitErr){result.ExitCode=exitErr.ExitCode()}
		if ctx.Err()==context.DeadlineExceeded{result.ExitCode=124;result.Output+="\nTask timed out."}
		if ctx.Err()==context.Canceled{result.ExitCode=130;result.Output+="\nTask cancelled."}
	}
	return result,nil
}
