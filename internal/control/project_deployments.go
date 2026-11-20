package control

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"veltrix/internal/shared"
)

type projectDeploymentRequest struct {
	DeviceID int64  `json:"device_id"`
	SiteID   string `json:"site_id"`
	Branch   string `json:"branch"`
	Commit   string `json:"commit_sha"`
	AllowProjectCode bool `json:"allow_project_code"`
}

func deploymentTerminal(state string) bool {
	switch state { case "succeeded","failed","cancelled": return true }
	return false
}

func deploymentStoredState(state string) string {
	switch state {
	case "healthy": return "succeeded"
	case "queued","preparing","cloning","installing","building","starting","failed","cancelled": return state
	default: return "running"
	}
}

func (s *Server) persistDeploymentJobEvent(ctx context.Context, userID, deviceID int64, event shared.DeploymentJobEvent) {
	if !validLogSiteID(event.SiteId) || event.JobId == "" || len(event.JobId) > 64 { return }
	var projectID sql.NullInt64
	var branch string
	_ = s.db.conn.QueryRowContext(ctx,`SELECT prs.project_id,prs.deploy_branch FROM project_repository_sites prs WHERE prs.device_id=? AND prs.site_id=? LIMIT 1`,deviceID,event.SiteId).Scan(&projectID,&branch)
	if !projectID.Valid {
		_ = s.db.conn.QueryRowContext(ctx,`SELECT ps.project_id,COALESCE(st.git_branch,'') FROM project_sites ps JOIN projects p ON p.id=ps.project_id JOIN sites st ON st.device_id=ps.device_id AND st.site_id=ps.site_id WHERE ps.device_id=? AND ps.site_id=? AND p.user_id=? ORDER BY ps.project_id LIMIT 1`,deviceID,event.SiteId,userID).Scan(&projectID,&branch)
	}
	state:=deploymentStoredState(event.State)
	message:=strings.TrimSpace(event.Message);if len(message)>1000{message=message[:1000]}
	var runID int64
	err:=s.db.conn.QueryRowContext(ctx,`SELECT id FROM project_deployment_runs WHERE device_id=? AND site_id=? AND status IN ('queued','running','preparing','cloning','installing','building','starting') AND (agent_job_id='' OR agent_job_id=?) ORDER BY id DESC LIMIT 1`,deviceID,event.SiteId,event.JobId).Scan(&runID)
	if err==sql.ErrNoRows {
		res,insertErr:=s.db.conn.ExecContext(ctx,`INSERT INTO project_deployment_runs(project_id,user_id,device_id,site_id,source,branch,agent_job_id,status,progress,message,started_at,finished_at) VALUES(?,?,?,?, 'agent',?,?,?,?,?,IF(?='queued',NULL,UTC_TIMESTAMP()),IF(? IN ('succeeded','failed','cancelled'),UTC_TIMESTAMP(),NULL))`,nullableInt64(projectID),userID,deviceID,event.SiteId,branch,event.JobId,state,event.Progress,message,state,state)
		if insertErr!=nil{return};runID,_=res.LastInsertId()
	} else if err==nil {
		_,_=s.db.conn.ExecContext(ctx,`UPDATE project_deployment_runs SET agent_job_id=?,status=?,progress=?,message=?,output=LEFT(CONCAT(COALESCE(output,''),IF(?='', '', CONCAT(?, '\\n'))),65535),started_at=COALESCE(started_at,IF(?='queued',NULL,UTC_TIMESTAMP())),finished_at=IF(? IN ('succeeded','failed','cancelled'),UTC_TIMESTAMP(),NULL) WHERE id=?`,event.JobId,state,event.Progress,message,message,message,state,state,runID)
	}
	if runID>0 && event.CommitSHA!="" && validWebhookCommit(event.CommitSHA) {
		_,_=s.db.conn.ExecContext(ctx,`UPDATE project_deployment_runs SET commit_sha=? WHERE id=?`,event.CommitSHA,runID)
	}
}

func nullableInt64(value sql.NullInt64) interface{} {
	if value.Valid { return value.Int64 }
	return nil
}

func (s *Server) handleProjectDeployments(w http.ResponseWriter,r *http.Request) {
	u:=UserFromContext(r.Context())
	parts:=strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path,"/api/projects/"),"/"),"/")
	if len(parts)<2 || parts[1]!="deployments" || len(parts)>4 { http.NotFound(w,r);return }
	projectID,err:=strconv.ParseInt(parts[0],10,64);if err!=nil||projectID<1{apiError(w,shared.ErrBadRequest("invalid project id"));return}
	ownerID,accessErr:=s.authorizeProjectOwner(r.Context(),projectID,u.ID,"project.read");if accessErr!=nil{apiError(w,accessErr);return}
	if r.Method==http.MethodGet && len(parts)==2 {
		rows,err:=s.db.conn.QueryContext(r.Context(),`SELECT id,device_id,site_id,source,branch,commit_sha,agent_job_id,status,progress,message,created_at,started_at,finished_at FROM project_deployment_runs WHERE project_id=? ORDER BY id DESC LIMIT 100`,projectID)
		if err!=nil{apiError(w,shared.ErrInternal("could not load project deployment history"));return};defer rows.Close()
		out:=make([]map[string]interface{},0)
		for rows.Next(){var id,did int64;var sid,source,branch,commit,jobID,status,message string;var progress int;var created time.Time;var started,finished sql.NullTime
			if err:=rows.Scan(&id,&did,&sid,&source,&branch,&commit,&jobID,&status,&progress,&message,&created,&started,&finished);err!=nil{apiError(w,shared.ErrInternal("could not read project deployment history"));return}
			item:=map[string]interface{}{"id":id,"device_id":did,"site_id":sid,"source":source,"branch":branch,"commit_sha":commit,"agent_job_id":jobID,"status":status,"progress":progress,"message":message,"created_at":created,"started_at":nil,"finished_at":nil}
			if started.Valid{item["started_at"]=started.Time};if finished.Valid{item["finished_at"]=finished.Time};out=append(out,item)
		}
		if err:=rows.Err();err!=nil{apiError(w,shared.ErrInternal("could not read project deployment history"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"deployments":out});return
	}
	if len(parts)==2 && r.Method==http.MethodPost {
		var in projectDeploymentRequest
		if err:=decodeJSON(r,&in);err!=nil{apiError(w,shared.ErrBadRequest("invalid deployment request"));return}
		if !in.AllowProjectCode{apiError(w,shared.ErrForbidden("explicit consent to execute repository code is required"));return}
		id,queueErr:=s.queueProjectDeployment(r.Context(),u.ID,ownerID,projectID,in,"manual")
		if queueErr!=nil{apiError(w,queueErr);return}
		writeJSON(w,http.StatusAccepted,map[string]interface{}{"ok":true,"deployment_id":id,"status":"queued"});return
	}
	if len(parts)==3 && r.Method==http.MethodGet {
		runID,parseErr:=strconv.ParseInt(parts[2],10,64);if parseErr!=nil||runID<1{apiError(w,shared.ErrBadRequest("invalid deployment id"));return}
		var deviceID int64;var siteID,source,branch,commit,jobID,status,message string;var progress int;var output sql.NullString;var created time.Time
		err:=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,site_id,source,branch,commit_sha,agent_job_id,status,progress,message,output,created_at FROM project_deployment_runs WHERE id=? AND project_id=?`,runID,projectID).Scan(&deviceID,&siteID,&source,&branch,&commit,&jobID,&status,&progress,&message,&output,&created)
		if err!=nil{apiError(w,shared.ErrNotFound("deployment not found"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"deployment":map[string]interface{}{"id":runID,"device_id":deviceID,"site_id":siteID,"source":source,"branch":branch,"commit_sha":commit,"agent_job_id":jobID,"status":status,"progress":progress,"message":message,"output":output.String,"created_at":created}});return
	}
	if len(parts)==4 && r.Method==http.MethodPost {
		runID,parseErr:=strconv.ParseInt(parts[2],10,64);if parseErr!=nil||runID<1{apiError(w,shared.ErrBadRequest("invalid deployment id"));return}
		var deviceID int64;var siteID,branch,commit,jobID,status string
		err:=s.db.conn.QueryRowContext(r.Context(),`SELECT device_id,site_id,branch,commit_sha,agent_job_id,status FROM project_deployment_runs WHERE id=? AND project_id=?`,runID,projectID).Scan(&deviceID,&siteID,&branch,&commit,&jobID,&status)
		if err!=nil{apiError(w,shared.ErrNotFound("deployment not found"));return}
		switch parts[3] {
		case "cancel":
			if deploymentTerminal(status){apiError(w,shared.ErrBadRequest("deployment is already finished"));return}
			if jobID==""{apiError(w,shared.ErrBadRequest("Agent has not reported a cancellable deployment job yet"));return}
			conn,online:=s.hub.Get(deviceID);if !online{apiError(w,shared.ErrBadRequest("device is offline"));return}
			ctx,cancel:=context.WithTimeout(r.Context(),10*time.Second);defer cancel()
			result,err:=conn.SendCommand(ctx,"cancel_deployment_job",&shared.DeploymentJobAction{SiteId:siteID,JobId:jobID})
			if err!=nil||result==nil||!result.Ok{message:="could not cancel deployment";if err!=nil{message=err.Error()}else if result!=nil&&result.Message!=""{message=result.Message};apiError(w,shared.ErrBadRequest(message));return}
			writeJSON(w,http.StatusAccepted,map[string]interface{}{"ok":true,"message":"deployment cancellation requested"});return
		case "redeploy":
			id,queueErr:=s.queueProjectDeployment(r.Context(),u.ID,ownerID,projectID,projectDeploymentRequest{DeviceID:deviceID,SiteID:siteID,Branch:branch,Commit:commit,AllowProjectCode:true},"redeploy")
			if queueErr!=nil{apiError(w,queueErr);return}
			writeJSON(w,http.StatusAccepted,map[string]interface{}{"ok":true,"deployment_id":id,"status":"queued"});return
		default:http.NotFound(w,r);return
		}
	}
	http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
}

func (s *Server) queueProjectDeployment(ctx context.Context,actorID,ownerID,projectID int64,in projectDeploymentRequest,source string)(int64,error){
	if in.DeviceID<1||!validLogSiteID(in.SiteID){return 0,shared.ErrBadRequest("valid deployment target is required")}
	if !in.AllowProjectCode{return 0,shared.ErrForbidden("explicit project-code consent is required")}
	var fullName,branch string;var integrationID int64
	err:=s.db.conn.QueryRowContext(ctx,`SELECT pr.full_name,prs.deploy_branch,pr.integration_id FROM project_repository_sites prs JOIN project_repositories pr ON pr.project_id=prs.project_id WHERE prs.project_id=? AND prs.device_id=? AND prs.site_id=?`,projectID,in.DeviceID,in.SiteID).Scan(&fullName,&branch,&integrationID)
	if err!=nil{return 0,shared.ErrBadRequest("connect this project repository to the selected site first")}
	if strings.TrimSpace(in.Branch)!=""{branch=strings.TrimSpace(in.Branch)}
	if !gitBranchPattern.MatchString(branch)||strings.Contains(branch,"..")||strings.Contains(branch,"@{")||strings.HasSuffix(branch,"/")||strings.HasSuffix(branch,"."){return 0,shared.ErrBadRequest("invalid deployment branch")}
	if in.Commit!=""&&!releaseCommitPattern.MatchString(in.Commit){return 0,shared.ErrBadRequest("invalid commit SHA")}
	owner,name,_,err:=parseGitHubRepositoryName(fullName);if err!=nil{return 0,shared.ErrBadRequest("invalid connected repository")}
	token,err:=s.githubAppTokenForIntegration(ctx,ownerID,integrationID,owner);if err!=nil{return 0,shared.ErrForbidden("GitHub App installation is unavailable")}
	apiCtx,cancel:=context.WithTimeout(ctx,20*time.Second);defer cancel()
	branchInfo,err:=s.fetchGitHubBranch(apiCtx,token,owner,name,branch);if err!=nil{return 0,shared.ErrBadRequest("deployment branch is not accessible")}
	if in.Commit==""{in.Commit=branchInfo.Commit.SHA}
	if _,online:=s.hub.Get(in.DeviceID);!online{return 0,shared.ErrBadRequest("Android Agent is offline")}
	var sitePath,runtime string;var port int
	if err:=s.db.conn.QueryRowContext(ctx,`SELECT path,runtime,port FROM sites WHERE device_id=? AND site_id=?`,in.DeviceID,in.SiteID).Scan(&sitePath,&runtime,&port);err!=nil{return 0,shared.ErrNotFound("site not found")}
	gitCfg,err:=s.siteGitSettings(ctx,in.DeviceID,in.SiteID);if err!=nil{return 0,shared.ErrInternal("could not load site Git settings")}
	projectIDScope,environmentID,err:=s.siteEnvironmentScope(ctx,ownerID,in.DeviceID,in.SiteID);if err!=nil{return 0,err};if projectIDScope!=projectID{return 0,shared.ErrBadRequest("deployment target is not assigned to the selected project environment")}
	env,err:=s.siteRuntimeEnvironment(ctx,ownerID,in.DeviceID,in.SiteID,projectIDScope,environmentID);if err!=nil{return 0,err}
	if _,err:=s.db.conn.ExecContext(ctx,`UPDATE sites SET git_branch=? WHERE device_id=? AND site_id=?`,branch,in.DeviceID,in.SiteID);err!=nil{return 0,shared.ErrInternal("could not save deployment branch")}
	_ = s.db.conn.QueryRowContext(ctx,`UPDATE project_repository_sites SET deploy_branch=? WHERE project_id=? AND device_id=? AND site_id=?`,branch,projectID,in.DeviceID,in.SiteID)
	res,err:=s.db.conn.ExecContext(ctx,`INSERT INTO project_deployment_runs(project_id,user_id,device_id,site_id,source,branch,commit_sha,status,message) VALUES(?,?,?,?,?,?,?,'queued','Deployment queued')`,projectID,ownerID,in.DeviceID,in.SiteID,source,branch,in.Commit)
	if err!=nil{return 0,shared.ErrInternal("could not create deployment record")}
	runID,_:=res.LastInsertId()
	action:=&shared.SiteAction{SiteId:in.SiteID,Path:sitePath,Port:uint32(port),Runtime:runtime,GitURL:gitCfg.GitURL,GitBranch:branch,GitCommit:in.Commit,BuildCommand:gitCfg.BuildCommand,RootDir:gitCfg.RootDir,GitToken:gitCfg.GitToken,HealthCheckPath:gitCfg.HealthCheckPath,CloneTimeoutSeconds:gitCfg.CloneTimeoutSeconds,BuildTimeoutSeconds:gitCfg.BuildTimeoutSeconds,HealthCheckTimeoutSeconds:gitCfg.HealthCheckTimeoutSeconds,StartCommand:gitCfg.StartCommand,AllowProjectCode:true,Env:env}
	conn,online:=s.hub.Get(in.DeviceID);if !online||conn==nil{_,_=s.db.conn.ExecContext(ctx,`UPDATE project_deployment_runs SET status='failed',message='Agent disconnected before deployment started',finished_at=UTC_TIMESTAMP() WHERE id=?`,runID);return runID,nil}
	go s.runTrackedProjectDeployment(runID,ownerID,actorID,in.DeviceID,conn,action)
	return runID,nil
}

func (s *Server) runTrackedProjectDeployment(runID,ownerID,actorID,deviceID int64,conn *AgentConn,action *shared.SiteAction){
	ctx,cancel:=context.WithTimeout(context.Background(),3*time.Hour);defer cancel()
	result,err:=conn.SendCommand(ctx,"deploy",action)
	status,message:="succeeded","Deployment completed"
	var started shared.StartSiteResult
	if err!=nil||result==nil||!result.Ok{
		status="failed";message="Deployment failed"
		if err!=nil{message=err.Error()}else if result!=nil&&result.Message!=""{message=result.Message}
	}else{
		_ = shared.Unmarshal(result.Payload,&started)
		if started.CommitHash!=""{_,_=s.db.conn.ExecContext(context.Background(),`UPDATE sites SET deployed_commit=?,deployed_at=UTC_TIMESTAMP() WHERE device_id=? AND site_id=?`,started.CommitHash,deviceID,action.SiteId)}
	}
	if len(message)>1000{message=message[:1000]}
	_,_=s.db.conn.ExecContext(context.Background(),`UPDATE project_deployment_runs SET status=IF(status='cancelled','cancelled',?),progress=IF(status='cancelled',0,?),message=?,finished_at=UTC_TIMESTAMP(),commit_sha=IF(?='',commit_sha,?) WHERE id=?`,status,map[bool]int{true:100,false:0}[status=="succeeded"],message,started.CommitHash,started.CommitHash,runID)
	_ = s.db.Log(context.Background(),&shared.LogEntry{UserID:shared.PtrInt64(actorID),DeviceID:shared.PtrInt64(deviceID),Action:"project_deployment_"+status,Details:map[string]interface{}{"deployment_id":runID,"site_id":action.SiteId,"commit":started.CommitHash,"error":func()string{if status=="failed"{return message};return ""}()}})
	if status=="failed"{s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"error","Deployment failed: "+action.SiteId,message)}else{s.notifyWorkspaceEvent(context.Background(),ownerID,actorID,"success","Deployment completed: "+action.SiteId,message)}
}

func (s *Server) createWebhookDeploymentRun(ctx context.Context,userID,deviceID int64,siteID,branch,commit,delivery string) int64 {
	var projectID sql.NullInt64
	_ = s.db.conn.QueryRowContext(ctx,`SELECT project_id FROM project_repository_sites WHERE device_id=? AND site_id=? AND auto_deploy_enabled=TRUE LIMIT 1`,deviceID,siteID).Scan(&projectID)
	res,err:=s.db.conn.ExecContext(ctx,`INSERT INTO project_deployment_runs(project_id,user_id,device_id,site_id,source,branch,commit_sha,delivery_id,status,message) VALUES(?,?,?,?, 'github_push',?,?,?,'queued','GitHub push received')`,nullableInt64(projectID),userID,deviceID,siteID,branch,commit,delivery)
	if err!=nil{return 0};id,_:=res.LastInsertId();return id
}

func (s *Server) finishWebhookDeploymentRun(runID int64,status,message string){
	if runID<1{return};if len(message)>1000{message=message[:1000]}
	progress:=0;if status=="succeeded"{progress=100}
	_,_=s.db.conn.ExecContext(context.Background(),`UPDATE project_deployment_runs SET status=IF(status='cancelled','cancelled',?),progress=IF(status='cancelled',0,?),message=?,finished_at=UTC_TIMESTAMP() WHERE id=?`,status,progress,message,runID)
}
