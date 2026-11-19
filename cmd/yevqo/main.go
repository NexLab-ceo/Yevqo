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
		fmt.Fprintln(stderr, "usage: yevqo-cli [--server URL] [--session COOKIE] <deploy|stop|logs> <device-id> <site-id>")
		return 2
	}
	if len(rest) != 3 {
		fmt.Fprintln(stderr, "expected command, device-id and site-id")
		return 2
	}
	command, deviceID, siteID := rest[0], rest[1], rest[2]
	if deviceID == "" || siteID == "" || strings.ContainsAny(deviceID+siteID, "/?#") {
		fmt.Fprintln(stderr, "invalid device-id or site-id")
		return 2
	}
	client, err := newAPIClient(*server, *session)
	if err != nil { fmt.Fprintln(stderr, err); return 2 }
	var result apiResponse
	switch command {
	case "deploy":
		result, err = client.post("/api/sites/"+url.PathEscape(deviceID)+"/"+url.PathEscape(siteID)+"/cmd", map[string]any{"action":"deploy", "allow_project_code":true})
	case "stop":
		result, err = client.post("/api/sites/"+url.PathEscape(deviceID)+"/"+url.PathEscape(siteID)+"/cmd", map[string]any{"action":"stop"})
	case "logs":
		result, err = client.post("/api/devices/"+url.PathEscape(deviceID)+"/cmd", map[string]any{"action":"get_logs", "payload":map[string]any{"site_id":siteID, "lines":200}})
	default:
		fmt.Fprintf(stderr, "unknown command %q (expected deploy, stop or logs)\n", command)
		return 2
	}
	if err != nil { fmt.Fprintln(stderr, err); return 1 }
	if command == "logs" && len(result.Payload) > 0 {
		payloadBytes := result.Payload
		// Agent payloads are []byte and therefore arrive as a base64 JSON string.
		var encoded string
		if json.Unmarshal(payloadBytes, &encoded) == nil {
			if decoded, decodeErr := base64.StdEncoding.DecodeString(encoded); decodeErr == nil { payloadBytes = decoded }
		}
		var payload map[string]any
		if json.Unmarshal(payloadBytes, &payload) == nil {
			for _, key := range []string{"content", "logs", "text"} {
				if value, ok := payload[key].(string); ok { fmt.Fprint(stdout, value); return 0 }
			}
		}
		fmt.Fprintln(stdout, string(payloadBytes))
		return 0
	}
	if result.Message != "" { fmt.Fprintln(stdout, result.Message) } else { fmt.Fprintln(stdout, "ok") }
	return 0
}

func main() {
	if code := run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr); code != 0 { os.Exit(code) }
}
