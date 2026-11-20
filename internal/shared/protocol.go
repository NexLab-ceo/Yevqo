// Package shared — общие структуры для сервера и агента.
// Этот файл — вручную написанная замена сгенерированному protoc-коду.
package shared

import ("encoding/json"; "time")

// ============================================================
// ENVELOPE — обёртка для всех сообщений
// ============================================================

type Envelope struct {
	AuthReq  *AuthRequest
	AuthResp *AuthResponse
	Command  *Command
	Result   *Result
	Event    *Event
}

// ============================================================
// АУТЕНТИФИКАЦИЯ
// ============================================================

type AuthRequest struct {
	Token        string
	DeviceId     string
	Hostname     string
	TailscaleIp  string
	AgentVersion string
	Platform string
	Architecture string
	Capabilities []string
}

type AuthResponse struct {
	Ok      bool
	Message string
	UserId  int64
}

// ============================================================
// КОМАНДЫ (Сервер -> Агент)
// ============================================================

type Command struct {
	Id      string
	Action  string
	Payload []byte
}

type InstallDatabaseServer struct {
	ServerID int64
	Engine string
	Port uint32
	DataDirectory string
	MaxConnections uint32
	SSLMode string `json:"ssl_mode"`
}

type DatabaseServerAction struct {
	SSLMode string `json:"ssl_mode,omitempty"`
	MaxConnections uint32 `json:"max_connections,omitempty"`
	Engine string `json:"engine,omitempty"`
	ServerID int64 `json:"server_id"`
	Port uint32 `json:"port"`
	DataDirectory string `json:"data_directory"`
}

type DatabaseServerInstallResult struct {
	Version string `json:"version"`
	DataDirectory string `json:"data_directory"`
	Port uint32 `json:"port"`
}

type CreateDatabase struct {
	Engine string `json:"engine,omitempty"`
	DataDirectory string `json:"data_directory,omitempty"`
	DatabaseID int64 `json:"database_id"`
	ServerID int64 `json:"server_id"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	Port uint32 `json:"port"`
}

type ListDatabaseRoles struct {
	Engine string `json:"engine,omitempty"`
	DataDirectory string `json:"data_directory,omitempty"`
	ServerID int64 `json:"server_id"`
	Port uint32 `json:"port"`
}

type DatabaseRole struct {
	Name string `json:"name"`
	CanLogin bool `json:"can_login"`
	IsSuperuser bool `json:"is_superuser"`
	IsReplication bool `json:"is_replication"`
	BypassRLS bool `json:"bypass_rls"`
	Inherit bool `json:"inherit"`
	ConnectionLimit int `json:"connection_limit"`
}

type DatabaseRolesResult struct {
	Roles []DatabaseRole `json:"roles"`
}

type GetDatabaseStats struct { Engine string `json:"engine,omitempty"`
	DataDirectory string `json:"data_directory,omitempty"`; ServerID int64 `json:"server_id"`; Port uint32 `json:"port"`; Database string `json:"database"` }
type DatabaseStats struct { SizeBytes int64 `json:"size_bytes"`; ActiveConnections int `json:"active_connections"`; MaxConnections int `json:"max_connections"`; TransactionsCommitted int64 `json:"transactions_committed"`; TransactionsRolledBack int64 `json:"transactions_rolled_back"`; Deadlocks int64 `json:"deadlocks"`; CapturedAt time.Time `json:"captured_at"` }

type ManageDatabasePrivilege struct {
 Engine string `json:"engine,omitempty"`
	DataDirectory string `json:"data_directory,omitempty"`
 ServerID int64 `json:"server_id"`
 Port uint32 `json:"port"`
 Database string `json:"database"`
 Role string `json:"role"`
 Privilege string `json:"privilege"`
 Scope string `json:"scope"`
 Schema string `json:"schema,omitempty"`
 Action string `json:"action"`
}

type ManageDatabaseRole struct {
	Engine string `json:"engine,omitempty"`
	DataDirectory string `json:"data_directory,omitempty"`
	ServerID int64 `json:"server_id"`
	Port uint32 `json:"port"`
	Name string `json:"name"`
	Password string `json:"password,omitempty"`
}

type WriteBackupChunk struct {
	TransferID string `json:"transfer_id"`
	ResourceType string `json:"resource_type"`
	SiteId string `json:"site_id,omitempty"`
	ServerID int64 `json:"server_id,omitempty"`
	FileName string `json:"file_name"`
	Offset int64 `json:"offset"`
	TotalSize int64 `json:"total_size"`
	Data []byte `json:"data,omitempty"`
	Final bool `json:"final,omitempty"`
	Abort bool `json:"abort,omitempty"`
}
type WriteBackupChunkResult struct { Received int64 `json:"received"`; Complete bool `json:"complete"` }

type RestoreSiteBackup struct {
	SiteId string `json:"site_id"`
	FileName string `json:"file_name"`
}

type ReadSiteBackupChunk struct {
	SiteId string `json:"site_id"`
	FileName string `json:"file_name"`
	Offset int64 `json:"offset"`
	Length int `json:"length"`
}

type PruneBackups struct {
	SiteId string `json:"site_id,omitempty"`
	ServerID int64 `json:"server_id,omitempty"`
	DatabaseID int64 `json:"database_id,omitempty"`
	Keep int `json:"keep"`
}
type PruneBackupsResult struct { Removed []string `json:"removed"` }

type CreateSiteBackup struct { SiteId string }

type CreateDatabaseBackup struct {
	Engine string `json:"engine,omitempty"`
	DataDirectory string `json:"data_directory,omitempty"`
	DatabaseID int64 `json:"database_id"`
	ServerID int64 `json:"server_id"`
	Database string `json:"database"`
	Port uint32 `json:"port"`
}

type TestDatabaseConnection struct {
	Engine string `json:"engine,omitempty"`
	DataDirectory string `json:"data_directory,omitempty"`
	DatabaseID int64 `json:"database_id"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	Port uint32 `json:"port"`
	SSLMode string `json:"ssl_mode"`
}

type RestoreDatabaseBackup struct {
	BackupServerID int64 `json:"backup_server_id"`
	BackupFileName string `json:"backup_file_name"`
	Database CreateDatabase `json:"database"`
}

type ReadDatabaseBackupChunk struct { ServerID int64 `json:"server_id"`; FileName string `json:"file_name"`; Offset int64 `json:"offset"`; Length int `json:"length"` }
type DatabaseBackupChunk struct { Data []byte `json:"data"`; TotalSize int64 `json:"total_size"`; Offset int64 `json:"offset"`; EOF bool `json:"eof"` }

type DatabaseBackupResult struct {
	FileName string `json:"file_name"`
	SizeBytes int64 `json:"size_bytes"`
	Format string `json:"format"`
	Verified bool `json:"verified"`
}

type SiteBackupResult struct {
	FileName string `json:"file_name"`
	SizeBytes int64 `json:"size_bytes"`
}

type GetMetrics struct{}

type MetricsResult struct {
	CPUPercent float64 `json:"cpu_percent"`
	Load1m float64 `json:"load_1m"`
	MemoryTotalBytes uint64 `json:"memory_total_bytes"`
	MemoryAvailableBytes uint64 `json:"memory_available_bytes"`
	StorageTotalBytes uint64 `json:"storage_total_bytes"`
	StorageAvailableBytes uint64 `json:"storage_available_bytes"`
	TemperatureCelsius *float64 `json:"temperature_celsius,omitempty"`
	CapturedAt time.Time `json:"captured_at"`
}

type AnalyzeSiteBuild struct {
	SiteId string `json:"site_id"`
	RootDir string `json:"root_dir,omitempty"`
}

type AndroidBuildSite struct {
	SiteId string `json:"site_id"`
	RootDir string `json:"root_dir,omitempty"`
	Stack string `json:"stack"`
	PackageTool string `json:"package_tool,omitempty"`
	InstallCommand string `json:"install_command,omitempty"`
	BuildCommand string `json:"build_command,omitempty"`
	AllowProjectCode bool `json:"allow_project_code"`
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
	MaxLogBytes int `json:"max_log_bytes,omitempty"`
}

type ListBuildHistory struct {
	SiteId string `json:"site_id"`
	Limit int `json:"limit,omitempty"`
}

type GetBuildHistory struct {
	SiteId string `json:"site_id"`
	ID string `json:"id"`
}

type ListBuildArtifacts struct {
 SiteId string `json:"site_id"`
 BuildID string `json:"build_id"`
}
type ReadBuildArtifactChunk struct {
 SiteId string `json:"site_id"`
 BuildID string `json:"build_id"`
 Path string `json:"path"`
 Offset int64 `json:"offset"`
 Limit int `json:"limit,omitempty"`
}

// PreviewDeploy is intentionally separate from production SiteAction routing.
type PreviewDeploy struct {
	PreviewID string `json:"preview_id"`
	SiteAction
}

type PreviewRemove struct {
	PreviewID string `json:"preview_id"`
}

type CreateSite struct {
	SiteId string `json:"site_id"`
	Port uint32 `json:"port"`
	Runtime string `json:"runtime"`
	TemplateID string `json:"template_id,omitempty"`
}

type StartSite struct {
	SiteId  string            `json:"site_id"`
	StartCommand string `json:"start_command,omitempty"`
	Path    string            `json:"path"`
	Port    uint32            `json:"port"`
	Runtime string            `json:"runtime"`
	Env     map[string]string `json:"env,omitempty"`
}

type StopSite struct {
	SiteId string `json:"site_id"`
}

type GetSiteRestartPolicy struct { SiteId string `json:"site_id"` }

type SetSiteRestartPolicy struct {
	SiteId string `json:"site_id"`
	Policy string `json:"policy"`
}

type GitPull struct {
	SiteId string `json:"site_id"`
}

type SiteAction struct {
	SiteId string `json:"site_id"`
	AllowProjectCode bool `json:"allow_project_code,omitempty"`
	Path string `json:"path"`
	Port uint32 `json:"port"`
	Runtime string `json:"runtime"`
	GitURL string `json:"git_url,omitempty"`
	GitBranch string `json:"git_branch,omitempty"`
	HealthCheckPath string `json:"health_check_path,omitempty"`
	CloneTimeoutSeconds int `json:"clone_timeout_seconds,omitempty"`
	BuildTimeoutSeconds int `json:"build_timeout_seconds,omitempty"`
	HealthCheckTimeoutSeconds int `json:"health_check_timeout_seconds,omitempty"`
	StartCommand string `json:"start_command,omitempty"`
	BuildCommand string `json:"build_command,omitempty"`
	RootDir string `json:"root_dir,omitempty"`
	GitToken string `json:"git_token,omitempty"`
	GitCommit string `json:"git_commit,omitempty"`
	Env map[string]string `json:"env,omitempty"`
}

type ListSiteReleases struct {
	SiteId string `json:"site_id"`
	Runtime string `json:"runtime,omitempty"`
}
type PruneSiteReleases struct {
	SiteId string `json:"site_id"`
	Keep int `json:"keep"`
}
type PruneSiteReleasesResult struct {
	Removed []string `json:"removed"`
}
type RollbackSiteRelease struct {
	SiteId string `json:"site_id"`
	StartCommand string `json:"start_command,omitempty"`
	Commit string `json:"commit"`
	RootDir string `json:"root_dir,omitempty"`
	Runtime string `json:"runtime"`
	Port uint32 `json:"port"`
	Env map[string]string `json:"env,omitempty"`
}
type SiteReleasesResult struct {
	Releases []SiteRelease `json:"releases"`
}
type SiteRelease struct {
	Commit string `json:"commit"`
	Date string `json:"date,omitempty"`
	Active bool `json:"active"`
	Runtime string `json:"runtime,omitempty"`
}

type RunScheduledTask struct {
	SiteId string `json:"site_id"`
	TaskID int64 `json:"task_id"`
	Command string `json:"command"`
	TimeoutSeconds int `json:"timeout_seconds"`
}
type ScheduledTaskResult struct {
	ExitCode int `json:"exit_code"`
	Output string `json:"output"`
	DurationMS int64 `json:"duration_ms"`
}

type GetSiteMetrics struct { SiteId string `json:"site_id"` }
type SiteMetricsResult struct {
	SiteId string `json:"site_id"`
	PID int `json:"pid"`
	ProcessCount int `json:"process_count"`
	CPUPercent float64 `json:"cpu_percent"`
	RSSBytes uint64 `json:"rss_bytes"`
	CapturedAt time.Time `json:"captured_at"`
}

type GetLogs struct {
	SiteId string `json:"site_id"`
	Lines  uint32 `json:"lines"`
}

type CreateTunnel struct {
	SiteId string `json:"site_id"`
	TunnelToken string `json:"tunnel_token,omitempty"`
	Hostname string `json:"hostname"`
	Port uint32 `json:"port"`
}

type StopTunnel struct {
	SiteId string `json:"site_id"`
}

// ============================================================
// РЕЗУЛЬТАТЫ (Агент -> Сервер)
// ============================================================

type Result struct {
	Id      string
	Ok      bool
	Message string
	Payload []byte
	ErrorCode string `json:"error_code,omitempty"`
	Retryable bool `json:"retryable,omitempty"`
}

type StartSiteResult struct {
	Pid  uint32 `json:"pid"`
	Port uint32 `json:"port"`
	CommitHash string `json:"commit_hash,omitempty"`
	CommitDate string `json:"commit_date,omitempty"`
}

type LogsResult struct {
	Content string
}

// ============================================================
// СОБЫТИЯ (Агент -> Сервер)
// ============================================================

type Event struct {
	Event   string
	Payload []byte
}

type LogEvent struct {
	SiteId string
	Line   string
}

type DeploymentJobEvent struct {
 SiteId string `json:"site_id"`
 JobId string `json:"job_id"`
 State string `json:"state"`
 Progress int `json:"progress"`
 Message string `json:"message,omitempty"`
 Timestamp time.Time `json:"timestamp"`
}

type DeploymentJobAction struct { SiteId string `json:"site_id"`; JobId string `json:"job_id"` }

type StatusEvent struct {
	SiteId string
	Status string
	Pid    uint32
}

type PingEvent struct{}

// ============================================================
// МЕТОДЫ — имитация proto.Message
// ============================================================

func (m *AuthRequest) Reset()         { *m = AuthRequest{} }
func (m *AuthRequest) String() string { return "AuthRequest" }
func (m *AuthRequest) ProtoMessage()  {}

func (m *AuthResponse) Reset()         { *m = AuthResponse{} }
func (m *AuthResponse) String() string { return "AuthResponse" }
func (m *AuthResponse) ProtoMessage()  {}

func (m *Command) Reset()         { *m = Command{} }
func (m *Command) String() string { return "Command" }
func (m *Command) ProtoMessage()  {}

func (m *StartSite) Reset()         { *m = StartSite{} }
func (m *StartSite) String() string { return "StartSite" }
func (m *StartSite) ProtoMessage()  {}

func (m *StopSite) Reset()         { *m = StopSite{} }
func (m *StopSite) String() string { return "StopSite" }
func (m *StopSite) ProtoMessage()  {}

func (m *GitPull) Reset()         { *m = GitPull{} }
func (m *GitPull) String() string { return "GitPull" }
func (m *GitPull) ProtoMessage()  {}

func (m *GetLogs) Reset()         { *m = GetLogs{} }
func (m *GetLogs) String() string { return "GetLogs" }
func (m *GetLogs) ProtoMessage()  {}

func (m *CreateTunnel) Reset()         { *m = CreateTunnel{} }
func (m *CreateTunnel) String() string { return "CreateTunnel" }
func (m *CreateTunnel) ProtoMessage()  {}

func (m *Result) Reset()         { *m = Result{} }
func (m *Result) String() string { return "Result" }
func (m *Result) ProtoMessage()  {}

func (m *StartSiteResult) Reset()         { *m = StartSiteResult{} }
func (m *StartSiteResult) String() string { return "StartSiteResult" }
func (m *StartSiteResult) ProtoMessage()  {}

func (m *LogsResult) Reset()         { *m = LogsResult{} }
func (m *LogsResult) String() string { return "LogsResult" }
func (m *LogsResult) ProtoMessage()  {}

func (m *Event) Reset()         { *m = Event{} }
func (m *Event) String() string { return "Event" }
func (m *Event) ProtoMessage()  {}

func (m *LogEvent) Reset()         { *m = LogEvent{} }
func (m *LogEvent) String() string { return "LogEvent" }
func (m *LogEvent) ProtoMessage()  {}

func (m *StatusEvent) Reset()         { *m = StatusEvent{} }
func (m *StatusEvent) String() string { return "StatusEvent" }
func (m *StatusEvent) ProtoMessage()  {}

func (m *PingEvent) Reset()         { *m = PingEvent{} }
func (m *PingEvent) String() string { return "PingEvent" }
func (m *PingEvent) ProtoMessage()  {}

// ============================================================
// СЕРИАЛИЗАЦИЯ (JSON вместо protobuf)
// ============================================================

func Marshal(m interface{}) ([]byte, error) {
	return json.Marshal(m)
}

func Unmarshal(data []byte, m interface{}) error {
	return json.Unmarshal(data, m)
}

// ============================================================
// ENVELOPE — определение типа
// ============================================================

func (e *Envelope) GetType() string {
	switch {
	case e.AuthReq != nil:
		return "auth_req"
	case e.AuthResp != nil:
		return "auth_resp"
	case e.Command != nil:
		return "command"
	case e.Result != nil:
		return "result"
	case e.Event != nil:
		return "event"
	default:
		return ""
	}
}
