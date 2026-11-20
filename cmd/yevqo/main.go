package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"strconv"
	"time"
)

type apiClient struct {
	baseURL string
	session string
	httpClient *http.Client
}

type apiResponse struct {
	OK bool `json:"ok"`
	Message string `json:"message"`
	Error string `json:"error"`
	Payload json.RawMessage `json:"payload"`
}

func newAPIClient(baseURL, session string) (*apiClient, error) {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("YEVQO_URL must be an absolute http(s) URL")
	}
	if strings.TrimSpace(session) == "" {
		return nil, errors.New("YEVQO_SESSION is required (session cookie value)")
	}
	return &apiClient{baseURL: strings.TrimRight(baseURL, "/"), session: session, httpClient: &http.Client{Timeout: 30 * time.Minute}}, nil
}

func (c *apiClient) get(path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil { return nil, err }
	req.AddCookie(&http.Cookie{Name:"veltrix_session", Value:c.session, Path:"/"})
	resp, err := c.httpClient.Do(req)
	if err != nil { return nil, fmt.Errorf("request Control Plane: %w", err) }
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil { return nil, err }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var result apiResponse
		if json.Unmarshal(data,&result)==nil && result.Error!="" { return nil, errors.New(result.Error) }
		return nil, fmt.Errorf("Control Plane returned HTTP %d: %s",resp.StatusCode,strings.TrimSpace(string(data)))
	}
	var result apiResponse
	if json.Unmarshal(data,&result)==nil && !result.OK {
		message:=result.Message;if message==""{message=result.Error};if message!=""{return nil,errors.New(message)}
	}
	return data,nil
}

func (c *apiClient) post(path string, body any) (apiResponse, error) {
	encoded, err := json.Marshal(body)
	if err != nil { return apiResponse{}, err }
	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewReader(encoded))
	if err != nil { return apiResponse{}, err }
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name:"veltrix_session", Value:c.session, Path:"/"})
	resp, err := c.httpClient.Do(req)
	if err != nil { return apiResponse{}, fmt.Errorf("request Control Plane: %w", err) }
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil { return apiResponse{}, err }
	var result apiResponse
	if err := json.Unmarshal(data, &result); err != nil { return result, fmt.Errorf("Control Plane returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data))) }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if result.Error == "" { result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode) }
		return result, errors.New(result.Error)
	}
	if !result.OK { message:=result.Message; if message=="" { message=result.Error }; if message=="" { message="operation failed" }; return result, errors.New(message) }
	return result, nil
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	global := flag.NewFlagSet("yevqo-cli", flag.ContinueOnError)
	global.SetOutput(stderr)
	server := global.String("server", getenv("YEVQO_URL"), "Control Plane base URL")
	session := global.String("session", getenv("YEVQO_SESSION"), "Control Plane session cookie")
	if err := global.Parse(args); err != nil { return 2 }
	rest := global.Args()
	if len(rest) == 0 {
		fmt.Fprintln(stderr, "usage: yevqo-cli [--server URL] [--session COOKIE] <projects|devices|sites|databases|database-servers|backups|backup-schedules|deployments|monitoring|project|environments|deploy|start|stop|restart|logs|backup-site|backup-db> [arguments]")
		return 2
	}
	command := rest[0]
	client, err := newAPIClient(*server, *session)
	if err != nil { fmt.Fprintln(stderr, err); return 2 }
	listPaths := map[string]string{
		"projects":"/api/projects",
		"devices":"/api/devices",
		"sites":"/api/sites",
		"databases":"/api/databases",
		"database-servers":"/api/databases/servers",
		"backups":"/api/backups",
		"backup-schedules":"/api/backup-schedules",
		"deployments":"/api/deployments",
		"monitoring":"/api/monitoring",
	}
	if path, ok := listPaths[command]; ok {
		if len(rest)!=1 { fmt.Fprintln(stderr,"this command takes no arguments");return 2 }
		data,err:=client.get(path);if err!=nil{fmt.Fprintln(stderr,err);return 1}
		var pretty bytes.Buffer
		if json.Indent(&pretty,data,"","  ")==nil { _,_=io.Copy(stdout,&pretty);fmt.Fprintln(stdout); } else { fmt.Fprintln(stdout,string(data)) }
		return 0
	}
	if command=="project"||command=="environments" {
		if len(rest)!=2 { fmt.Fprintln(stderr,"expected a project id");return 2 }
		if _,err:=strconv.ParseInt(rest[1],10,64);err!=nil { fmt.Fprintln(stderr,"invalid project id");return 2 }
		path:="/api/projects/"+url.PathEscape(rest[1])
		if command=="environments" { path+="/environments" }
		data,err:=client.get(path);if err!=nil{fmt.Fprintln(stderr,err);return 1}
		var pretty bytes.Buffer
		if json.Indent(&pretty,data,"","  ")==nil { _,_=io.Copy(stdout,&pretty);fmt.Fprintln(stdout); } else { fmt.Fprintln(stdout,string(data)) }
		return 0
	}
	if command=="backup-db" {
		if len(rest)!=2 { fmt.Fprintln(stderr,"expected a database id");return 2 }
		id,err:=strconv.ParseInt(rest[1],10,64);if err!=nil||id<1{fmt.Fprintln(stderr,"invalid database id");return 2}
		result,err:=client.post("/api/backups",map[string]any{"database_id":id})
		if err!=nil{fmt.Fprintln(stderr,err);return 1}
		if result.Message!=""{fmt.Fprintln(stdout,result.Message)}else{fmt.Fprintln(stdout,"backup queued")}
		return 0
	}
	if command=="backup-site" {
		if len(rest)!=3 { fmt.Fprintln(stderr,"expected device-id and site-id");return 2 }
		id,err:=strconv.ParseInt(rest[1],10,64);if err!=nil||id<1||strings.ContainsAny(rest[2],"/?#"){fmt.Fprintln(stderr,"invalid device-id or site-id");return 2}
		result,err:=client.post("/api/backups",map[string]any{"device_id":id,"site_id":rest[2]})
		if err!=nil{fmt.Fprintln(stderr,err);return 1}
		if result.Message!=""{fmt.Fprintln(stdout,result.Message)}else{fmt.Fprintln(stdout,"backup queued")}
		return 0
	}
	if command!="deploy"&&command!="start"&&command!="stop"&&command!="restart"&&command!="logs" {
		fmt.Fprintf(stderr,"unknown command %q\n",command);return 2
	}
	if len(rest)!=3 { fmt.Fprintln(stderr,"expected command, device-id and site-id");return 2 }
	deviceID,siteID:=rest[1],rest[2]
	if _,err:=strconv.ParseInt(deviceID,10,64);err!=nil||deviceID==""||siteID==""||strings.ContainsAny(deviceID+siteID,"/?#"){fmt.Fprintln(stderr,"invalid device-id or site-id");return 2}
	var result apiResponse
	switch command {
	case "deploy":
		result,err=client.post("/api/sites/"+url.PathEscape(deviceID)+"/"+url.PathEscape(siteID)+"/cmd",map[string]any{"action":"deploy","allow_project_code":true})
	case "start","stop","restart":
		result,err=client.post("/api/sites/"+url.PathEscape(deviceID)+"/"+url.PathEscape(siteID)+"/cmd",map[string]any{"action":command})
	case "logs":
		result,err=client.post("/api/devices/"+url.PathEscape(deviceID)+"/cmd",map[string]any{"action":"get_logs","payload":map[string]any{"site_id":siteID,"lines":200}})
	}
	if err!=nil{fmt.Fprintln(stderr,err);return 1}
	if command=="logs"&&len(result.Payload)>0 {
		payloadBytes:=result.Payload
		var encoded string
		if json.Unmarshal(payloadBytes,&encoded)==nil { if decoded,decodeErr:=base64.StdEncoding.DecodeString(encoded);decodeErr==nil{payloadBytes=decoded} }
		var payload map[string]any
		if json.Unmarshal(payloadBytes,&payload)==nil { for _,key:=range []string{"content","logs","text"}{if value,ok:=payload[key].(string);ok{fmt.Fprint(stdout,value);return 0}} }
		fmt.Fprintln(stdout,string(payloadBytes));return 0
	}
	if result.Message!=""{fmt.Fprintln(stdout,result.Message)}else{fmt.Fprintln(stdout,"ok")}
	return 0
}

func main() {
	if code := run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr); code != 0 { os.Exit(code) }
}
