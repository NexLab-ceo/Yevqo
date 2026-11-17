package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"veltrix/internal/shared"
)

const cloudflareAPIBase = "https://api.cloudflare.com/client/v4"
var cloudflareZoneIDPattern = regexp.MustCompile(`^[a-fA-F0-9]{32}package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"veltrix/internal/shared"
)

const cloudflareAPIBase = "https://api.cloudflare.com/client/v4"
)
var cloudflareTunnelIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type cloudflareAPIEnvelope struct {
	Success bool `json:"success"`
	Result json.RawMessage `json:"result"`
	Errors []struct { Message string `json:"message"` } `json:"errors"`
}

func cloudflareAPIRequest(ctx context.Context, token, method, endpoint string, body interface{}, target interface{}) error {
	var reader io.Reader
	if body != nil { encoded,err:=json.Marshal(body);if err!=nil{return err};reader=strings.NewReader(string(encoded)) }
	req,err:=http.NewRequestWithContext(ctx,method,cloudflareAPIBase+endpoint,reader);if err!=nil{return err}
	req.Header.Set("Authorization","Bearer "+token)
	req.Header.Set("Content-Type","application/json")
	req.Header.Set("Accept","application/json")
	resp,err:=integrationHTTPClient.Do(req);if err!=nil{return err};defer resp.Body.Close()
	var envelope cloudflareAPIEnvelope
	if err:=json.NewDecoder(io.LimitReader(resp.Body,2<<20)).Decode(&envelope);err!=nil{return err}
	if resp.StatusCode<200||resp.StatusCode>=300||!envelope.Success {
		if len(envelope.Errors)>0{return fmt.Errorf("Cloudflare API: %s",envelope.Errors[0].Message)}
		return fmt.Errorf("Cloudflare API returned HTTP %d",resp.StatusCode)
	}
	if target!=nil && len(envelope.Result)>0 { if err:=json.Unmarshal(envelope.Result,target);err!=nil{return err} }
	return nil
}

func validCloudflareZoneID(value string) bool { return cloudflareZoneIDPattern.MatchString(value) }
func validCloudflareTunnelID(value string) bool { return cloudflareTunnelIDPattern.MatchString(value) }
func hostnameBelongsToZone(hostname, zone string) bool {
	hostname=strings.ToLower(strings.TrimSuffix(strings.TrimSpace(hostname),"."))
	zone=strings.ToLower(strings.TrimSuffix(strings.TrimSpace(zone),"."))
	return hostname==zone || strings.HasSuffix(hostname,"."+zone)
}

func (s *Server) cloudflareCredentials(ctx context.Context,userID int64)(string,string,string,error){
	var zoneID,zoneName string;var nonce,ciphertext []byte
	err:=s.db.conn.QueryRowContext(ctx,`SELECT zone_id,zone_name,token_nonce,token_ciphertext FROM cloudflare_accounts WHERE user_id=?`,userID).Scan(&zoneID,&zoneName,&nonce,&ciphertext)
	if err!=nil{return "","","",err}
	token,err:=decryptCredential(s.credentialKey,nonce,ciphertext)
	if err!=nil{return "","","",err}
	return zoneID,zoneName,string(token),nil
}

func (s *Server) handleCloudflareConfig(w http.ResponseWriter,r *http.Request){
	u:=UserFromContext(r.Context())
	switch r.Method {
	case http.MethodGet:
		var zoneID,zoneName string
		err:=s.db.conn.QueryRowContext(r.Context(),`SELECT zone_id,zone_name FROM cloudflare_accounts WHERE user_id=?`,u.ID).Scan(&zoneID,&zoneName)
		if err==sql.ErrNoRows{writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"connected":false});return};if err!=nil{apiError(w,shared.ErrInternal("could not load Cloudflare configuration"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"connected":true,"zone_id":zoneID,"zone_name":zoneName})
	case http.MethodPost:
		var in struct{Token string `json:"token"`;ZoneID string `json:"zone_id"`}
		if err:=decodeJSON(r,&in);err!=nil{apiError(w,shared.ErrBadRequest("invalid Cloudflare configuration"));return}
		in.Token=strings.TrimSpace(in.Token);in.ZoneID=strings.TrimSpace(in.ZoneID)
		if len(in.Token)<20||len(in.Token)>4096||strings.ContainsAny(in.Token,"\r\n\x00")||!validCloudflareZoneID(in.ZoneID){apiError(w,shared.ErrBadRequest("enter a valid scoped API token and zone ID"));return}
		ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second);defer cancel()
		var verify struct{Status string `json:"status"`}
		if err:=cloudflareAPIRequest(ctx,in.Token,http.MethodGet,"/user/tokens/verify",nil,&verify);err!=nil||verify.Status!="active"{apiError(w,shared.ErrBadRequest("Cloudflare token is invalid or inactive"));return}
		var zone struct{ID string `json:"id"`;Name string `json:"name"`;Status string `json:"status"`}
		if err:=cloudflareAPIRequest(ctx,in.Token,http.MethodGet,"/zones/"+in.ZoneID,nil,&zone);err!=nil||zone.ID!=in.ZoneID||zone.Name==""||zone.Status!="active"{apiError(w,shared.ErrBadRequest("Cloudflare zone could not be verified"));return}
		nonce,ciphertext,err:=encryptCredential(s.credentialKey,[]byte(in.Token));if err!=nil{apiError(w,shared.ErrInternal("could not encrypt Cloudflare token"));return}
		_,err=s.db.conn.ExecContext(r.Context(),`INSERT INTO cloudflare_accounts(user_id,zone_id,zone_name,token_nonce,token_ciphertext,last_verified_at) VALUES(?,?,?,?,?,UTC_TIMESTAMP()) ON DUPLICATE KEY UPDATE zone_id=VALUES(zone_id),zone_name=VALUES(zone_name),token_nonce=VALUES(token_nonce),token_ciphertext=VALUES(token_ciphertext),last_verified_at=VALUES(last_verified_at)`,u.ID,in.ZoneID,zone.Name,nonce,ciphertext)
		if err!=nil{apiError(w,shared.ErrInternal("could not save Cloudflare configuration"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"connected":true,"zone_id":zone.ID,"zone_name":zone.Name})
	case http.MethodDelete:
		_,err:=s.db.conn.ExecContext(r.Context(),`DELETE FROM cloudflare_accounts WHERE user_id=?`,u.ID)
		if err!=nil{apiError(w,shared.ErrInternal("could not disconnect Cloudflare"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"connected":false})
	default:http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}

type cloudflareDNSRecord struct {
	ID string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
	Content string `json:"content"`
	Proxied bool `json:"proxied"`
	TTL int `json:"ttl"`
}

func (s *Server) handleCloudflareDNS(w http.ResponseWriter,r *http.Request){
	u:=UserFromContext(r.Context())
	zoneID,zoneName,token,err:=s.cloudflareCredentials(r.Context(),u.ID)
	if err!=nil{apiError(w,shared.ErrBadRequest("connect and verify a Cloudflare zone first"));return}
	ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second);defer cancel()
	switch r.Method {
	case http.MethodGet:
		var records []cloudflareDNSRecord
		err:=cloudflareAPIRequest(ctx,token,http.MethodGet,"/zones/"+zoneID+"/dns_records?per_page=100",nil,&records)
		if err!=nil{apiError(w,shared.ErrBadRequest("could not load Cloudflare DNS records"));return}
		managed:=make([]cloudflareDNSRecord,0)
		for _,record:=range records{if record.Type=="CNAME"&&strings.HasSuffix(strings.ToLower(record.Content),".cfargotunnel.com"){managed=append(managed,record)}}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"records":managed,"zone_name":zoneName})
	case http.MethodPost:
		var in struct{Hostname string `json:"hostname"`;TunnelID string `json:"tunnel_id"`}
		if err:=decodeJSON(r,&in);err!=nil{apiError(w,shared.ErrBadRequest("invalid DNS record"));return}
		in.Hostname=strings.ToLower(strings.TrimSuffix(strings.TrimSpace(in.Hostname),"."))
		in.TunnelID=strings.TrimSpace(in.TunnelID)
		if !domainPattern.MatchString(in.Hostname)||!hostnameBelongsToZone(in.Hostname,zoneName)||!validCloudflareTunnelID(in.TunnelID){apiError(w,shared.ErrBadRequest("hostname must belong to the configured zone and tunnel ID must be a UUID"));return}
		var existing []cloudflareDNSRecord
		err:=cloudflareAPIRequest(ctx,token,http.MethodGet,"/zones/"+zoneID+"/dns_records?type=CNAME&name="+in.Hostname,nil,&existing)
		if err!=nil{apiError(w,shared.ErrBadRequest("could not check existing DNS record"));return}
		payload:=map[string]interface{}{"type":"CNAME","name":in.Hostname,"content":strings.ToLower(in.TunnelID)+".cfargotunnel.com","proxied":true,"ttl":1}
		method,endpoint:=http.MethodPost,"/zones/"+zoneID+"/dns_records"
		if len(existing)>0 {
			if existing[0].Type!="CNAME"||!strings.HasSuffix(strings.ToLower(existing[0].Content),".cfargotunnel.com"){apiError(w,shared.ErrBadRequest("a non-tunnel DNS record already uses this hostname; refusing to overwrite it"));return}
			method=http.MethodPut;endpoint+="/"+existing[0].ID
		}
		var record cloudflareDNSRecord
		if err:=cloudflareAPIRequest(ctx,token,method,endpoint,payload,&record);err!=nil{apiError(w,shared.ErrBadRequest("Cloudflare could not create the tunnel DNS record"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"record":record})
	case http.MethodDelete:
		var in struct{RecordID string `json:"record_id"`}
		if err:=decodeJSON(r,&in);err!=nil||len(in.RecordID)<10||len(in.RecordID)>64{apiError(w,shared.ErrBadRequest("invalid DNS record id"));return}
		var record cloudflareDNSRecord
		if err:=cloudflareAPIRequest(ctx,token,http.MethodGet,"/zones/"+zoneID+"/dns_records/"+in.RecordID,nil,&record);err!=nil||record.ID!=in.RecordID||record.Type!="CNAME"||!strings.HasSuffix(strings.ToLower(record.Content),".cfargotunnel.com")||!hostnameBelongsToZone(record.Name,zoneName){apiError(w,shared.ErrNotFound("DNS record not found in configured zone"));return}
		if err:=cloudflareAPIRequest(ctx,token,http.MethodDelete,"/zones/"+zoneID+"/dns_records/"+in.RecordID,nil,nil);err!=nil{apiError(w,shared.ErrBadRequest("could not delete DNS record"));return}
		writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true})
	default:http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
	}
}
