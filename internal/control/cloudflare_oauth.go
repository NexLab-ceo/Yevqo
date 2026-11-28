package control

import (
 "context"
 "crypto/rand"
 "crypto/sha256"
 "database/sql"
 "encoding/hex"
 "encoding/json"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "os"
 "regexp"
 "strconv"
 "strings"
 "time"

 "veltrix/internal/shared"
)

const (
 cloudflareOAuthAuthorizeURL = "https://dash.cloudflare.com/oauth2/auth"
 cloudflareOAuthTokenURL = "https://dash.cloudflare.com/oauth2/token"
 cloudflareOAuthScopes = "zone.read dns.write argotunnel.read argotunnel.write offline_access"
)

type cloudflareOAuthClientConfig struct { ClientID, ClientSecret, RedirectURI, AuthMethod string }
type cloudflareOAuthTokenResponse struct {
 AccessToken string `json:"access_token"`
 RefreshToken string `json:"refresh_token"`
 ExpiresIn int64 `json:"expires_in"`
}

func (s *Server) cloudflareOAuthClient(ctx context.Context) (cloudflareOAuthClientConfig,error) {
 var cfg cloudflareOAuthClientConfig
 var publicURL,clientID,authMethod string
 var nonce,ciphertext []byte
 err:=s.db.conn.QueryRowContext(ctx,`SELECT public_url,cloudflare_oauth_client_id,cloudflare_oauth_token_auth_method,cloudflare_oauth_secret_nonce,cloudflare_oauth_secret_ciphertext FROM platform_settings WHERE id=1`).Scan(&publicURL,&clientID,&authMethod,&nonce,&ciphertext)
 if err!=nil && err!=sql.ErrNoRows{return cfg,err}
 if strings.TrimSpace(publicURL)==""{publicURL=strings.TrimRight(strings.TrimSpace(os.Getenv("YEVQO_PUBLIC_URL")),"/")}
 if strings.TrimSpace(clientID)==""{clientID=strings.TrimSpace(os.Getenv("YEVQO_CLOUDFLARE_OAUTH_CLIENT_ID"))}
 secret:=""
 if len(nonce)>0&&len(ciphertext)>0{raw,e:=decryptCredential(s.credentialKey,nonce,ciphertext);if e!=nil{return cfg,e};secret=string(raw)}
 if secret==""{secret=os.Getenv("YEVQO_CLOUDFLARE_OAUTH_CLIENT_SECRET")}
 if authMethod!="client_secret_basic"&&authMethod!="client_secret_post"{authMethod="client_secret_post"}
 base,e:=normalizePlatformURL(publicURL);if e!=nil{return cfg,fmt.Errorf("Cloudflare OAuth public URL is not configured")}
 if clientID==""||secret==""{return cfg,fmt.Errorf("Cloudflare OAuth client credentials are not configured")}
 return cloudflareOAuthClientConfig{ClientID:clientID,ClientSecret:secret,RedirectURI:base+"/oauth/cloudflare/callback",AuthMethod:authMethod},nil
}

func (s *Server) handleCloudflareOAuthStart(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodPost{http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return}
 u:=UserFromContext(r.Context())
 var in struct{ReturnTo string `json:"return_to"`}
 if decodeJSON(r,&in)!=nil{apiError(w,shared.ErrBadRequest("invalid Cloudflare OAuth request"));return}
 returnTo:=strings.TrimSpace(in.ReturnTo)
 if returnTo==""{returnTo="/settings"}
 if len(returnTo)>1024||(returnTo!="/settings"&&returnTo!="/settings/integrations/cloudflare"&&!strings.HasPrefix(returnTo,"/sites/"))||strings.HasPrefix(returnTo,"//")||strings.ContainsAny(returnTo,"\r\n\x00"){apiError(w,shared.ErrBadRequest("invalid return path"));return}
 parsed,e:=url.ParseRequestURI(returnTo);if e!=nil||parsed.IsAbs()||parsed.Host!=""{apiError(w,shared.ErrBadRequest("invalid return path"));return}
 cfg,e:=s.cloudflareOAuthClient(r.Context());if e!=nil{apiError(w,shared.ErrBadRequest("Cloudflare OAuth is not configured by the platform administrator"));return}
 raw:=make([]byte,32);if _,e=rand.Read(raw);e!=nil{apiError(w,shared.ErrInternal("could not create Cloudflare OAuth state"));return}
 state:=hex.EncodeToString(raw);digest:=sha256.Sum256([]byte(state))
 _,_=s.db.conn.ExecContext(r.Context(),`DELETE FROM cloudflare_oauth_states WHERE expires_at<UTC_TIMESTAMP()`)
 _,e=s.db.conn.ExecContext(r.Context(),`INSERT INTO cloudflare_oauth_states(state_hash,user_id,return_to,expires_at) VALUES(?,?,?,?)`,hex.EncodeToString(digest[:]),u.ID,returnTo,time.Now().UTC().Add(10*time.Minute))
 if e!=nil{apiError(w,shared.ErrInternal("could not save Cloudflare OAuth state"));return}
 q:=url.Values{};q.Set("client_id",cfg.ClientID);q.Set("redirect_uri",cfg.RedirectURI);q.Set("response_type","code");q.Set("scope",cloudflareOAuthScopes);q.Set("state",state)
 writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"authorization_url":cloudflareOAuthAuthorizeURL+"?"+q.Encode()})
}

func (s *Server) cloudflareOAuthTokenRequest(ctx context.Context,cfg cloudflareOAuthClientConfig,form url.Values)(cloudflareOAuthTokenResponse,error){
 var out cloudflareOAuthTokenResponse
 if cfg.AuthMethod=="client_secret_post"{form.Set("client_id",cfg.ClientID);form.Set("client_secret",cfg.ClientSecret)}
 req,e:=http.NewRequestWithContext(ctx,http.MethodPost,cloudflareOAuthTokenURL,strings.NewReader(form.Encode()));if e!=nil{return out,e}
 req.Header.Set("Accept","application/json");req.Header.Set("Content-Type","application/x-www-form-urlencoded")
 if cfg.AuthMethod=="client_secret_basic"{req.SetBasicAuth(cfg.ClientID,cfg.ClientSecret)}
 resp,e:=integrationHTTPClient.Do(req);if e!=nil{return out,e};defer resp.Body.Close()
 body,e:=io.ReadAll(io.LimitReader(resp.Body,1<<20));if e!=nil{return out,e}
 if resp.StatusCode<200||resp.StatusCode>=300{return out,fmt.Errorf("Cloudflare OAuth token endpoint returned HTTP %d",resp.StatusCode)}
 if e=json.Unmarshal(body,&out);e!=nil{return out,e}
 if out.AccessToken==""{return out,fmt.Errorf("Cloudflare OAuth returned no access token")}
 return out,nil
}

func (s *Server) handleCloudflareOAuthCallback(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet{http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return}
 state:=r.URL.Query().Get("state");raw,e:=hex.DecodeString(state)
 if len(state)!=64||e!=nil||len(raw)!=32{http.Error(w,"Invalid Cloudflare OAuth state",http.StatusBadRequest);return}
 digest:=sha256.Sum256([]byte(state))
 tx,e:=s.db.conn.BeginTx(r.Context(),nil);if e!=nil{http.Error(w,"Cloudflare OAuth is temporarily unavailable",http.StatusInternalServerError);return}
 var userID int64;var returnTo string;var expires time.Time
 e=tx.QueryRowContext(r.Context(),`SELECT user_id,return_to,expires_at FROM cloudflare_oauth_states WHERE state_hash=? FOR UPDATE`,hex.EncodeToString(digest[:])).Scan(&userID,&returnTo,&expires)
 if e!=nil||time.Now().UTC().After(expires){_=tx.Rollback();http.Error(w,"Cloudflare OAuth state expired. Start again from Yevqo.",http.StatusBadRequest);return}
 if _,e=tx.ExecContext(r.Context(),`DELETE FROM cloudflare_oauth_states WHERE state_hash=?`,hex.EncodeToString(digest[:]));e!=nil{_=tx.Rollback();http.Error(w,"Could not consume Cloudflare OAuth state",http.StatusInternalServerError);return}
 if e=tx.Commit();e!=nil{http.Error(w,"Could not consume Cloudflare OAuth state",http.StatusInternalServerError);return}
 redirect:=func(status string,reason string){target,parseErr:=url.Parse(returnTo);if parseErr!=nil{target=&url.URL{Path:"/settings"}};q:=target.Query();q.Set("cloudflare_oauth",status);if reason!=""{q.Set("cloudflare_oauth_error",reason)};target.RawQuery=q.Encode();http.Redirect(w,r,target.String(),http.StatusSeeOther)}
 if oauthErr:=strings.TrimSpace(r.URL.Query().Get("error"));oauthErr!=""{if len(oauthErr)>64{oauthErr=oauthErr[:64]};if !regexp.MustCompile("^[A-Za-z0-9_.-]{1,64}$").MatchString(oauthErr){oauthErr="provider_error"};redirect("denied",oauthErr);return}
 code:=strings.TrimSpace(r.URL.Query().Get("code"));if code==""{redirect("error","");return}
 cfg,e:=s.cloudflareOAuthClient(r.Context());if e!=nil{redirect("error","");return}
 ctx,cancel:=context.WithTimeout(r.Context(),20*time.Second);defer cancel()
 tokens,e:=s.cloudflareOAuthTokenRequest(ctx,cfg,url.Values{"grant_type":{"authorization_code"},"code":{code},"redirect_uri":{cfg.RedirectURI}})
 if e!=nil||tokens.RefreshToken==""||tokens.ExpiresIn<60{redirect("error","");return}
 an,ac,e:=encryptCredential(s.credentialKey,[]byte(tokens.AccessToken));if e!=nil{redirect("error","");return}
 rn,rc,e:=encryptCredential(s.credentialKey,[]byte(tokens.RefreshToken));if e!=nil{redirect("error","");return}
 expiry:=time.Now().UTC().Add(time.Duration(tokens.ExpiresIn)*time.Second)
 _,e=s.db.conn.ExecContext(r.Context(),`INSERT INTO cloudflare_accounts(user_id,account_id,zone_id,zone_name,token_nonce,token_ciphertext,auth_method,oauth_refresh_nonce,oauth_refresh_ciphertext,oauth_expires_at,last_verified_at) VALUES(?,'','','',?,?,'oauth',?,?,?,UTC_TIMESTAMP()) ON DUPLICATE KEY UPDATE token_nonce=VALUES(token_nonce),token_ciphertext=VALUES(token_ciphertext),auth_method='oauth',oauth_refresh_nonce=VALUES(oauth_refresh_nonce),oauth_refresh_ciphertext=VALUES(oauth_refresh_ciphertext),oauth_expires_at=VALUES(oauth_expires_at),last_verified_at=VALUES(last_verified_at)`,userID,an,ac,rn,rc,expiry)
 if e!=nil{redirect("error","");return}
 redirect("connected","")
}

func (s *Server) cloudflareOAuthAccessToken(ctx context.Context,userID int64)(string,error){
 tx,e:=s.db.conn.BeginTx(ctx,nil);if e!=nil{return "",e};defer tx.Rollback()
 var method string;var an,ac,rn,rc []byte;var expiry sql.NullTime
 e=tx.QueryRowContext(ctx,`SELECT auth_method,token_nonce,token_ciphertext,oauth_refresh_nonce,oauth_refresh_ciphertext,oauth_expires_at FROM cloudflare_accounts WHERE user_id=? FOR UPDATE`,userID).Scan(&method,&an,&ac,&rn,&rc,&expiry)
 if e!=nil{return "",e}
 access,e:=decryptCredential(s.credentialKey,an,ac);if e!=nil{return "",e}
 if method!="oauth"||!expiry.Valid||time.Until(expiry.Time)>2*time.Minute{
  if e=tx.Commit();e!=nil{return "",e};return string(access),nil
 }
 if len(rn)==0||len(rc)==0{return "",fmt.Errorf("Cloudflare OAuth refresh token is unavailable; reconnect the account")}
 refresh,e:=decryptCredential(s.credentialKey,rn,rc);if e!=nil{return "",e}
 cfg,e:=s.cloudflareOAuthClient(ctx);if e!=nil{return "",e}
 work,cancel:=context.WithTimeout(ctx,20*time.Second);defer cancel()
 tokens,e:=s.cloudflareOAuthTokenRequest(work,cfg,url.Values{"grant_type":{"refresh_token"},"refresh_token":{string(refresh)}});if e!=nil{return "",e}
 if tokens.ExpiresIn<60{return "",fmt.Errorf("Cloudflare OAuth returned an invalid token lifetime")}
 nan,nac,e:=encryptCredential(s.credentialKey,[]byte(tokens.AccessToken));if e!=nil{return "",e}
 nrn,nrc:=rn,rc;if tokens.RefreshToken!=""{nrn,nrc,e=encryptCredential(s.credentialKey,[]byte(tokens.RefreshToken));if e!=nil{return "",e}}
 next:=time.Now().UTC().Add(time.Duration(tokens.ExpiresIn)*time.Second)
 _,e=tx.ExecContext(ctx,`UPDATE cloudflare_accounts SET token_nonce=?,token_ciphertext=?,oauth_refresh_nonce=?,oauth_refresh_ciphertext=?,oauth_expires_at=?,last_verified_at=UTC_TIMESTAMP() WHERE user_id=? AND auth_method='oauth'`,nan,nac,nrn,nrc,next,userID)
 if e!=nil{return "",e}
 if e=tx.Commit();e!=nil{return "",e}
 return tokens.AccessToken,nil
}

func (s *Server) cloudflareOAuthRevoke(ctx context.Context,cfg cloudflareOAuthClientConfig,token,tokenType string)error{
 form:=url.Values{"token":{token},"token_type_hint":{tokenType}}
 if cfg.AuthMethod=="client_secret_post"{form.Set("client_id",cfg.ClientID);form.Set("client_secret",cfg.ClientSecret)}
 req,e:=http.NewRequestWithContext(ctx,http.MethodPost,"https://dash.cloudflare.com/oauth2/revoke",strings.NewReader(form.Encode()));if e!=nil{return e}
 req.Header.Set("Accept","application/json");req.Header.Set("Content-Type","application/x-www-form-urlencoded")
 if cfg.AuthMethod=="client_secret_basic"{req.SetBasicAuth(cfg.ClientID,cfg.ClientSecret)}
 resp,e:=integrationHTTPClient.Do(req);if e!=nil{return e};defer resp.Body.Close()
 _,_=io.Copy(io.Discard,io.LimitReader(resp.Body,1<<20))
 if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("Cloudflare OAuth revoke endpoint returned HTTP %d",resp.StatusCode)}
 return nil
}

func (s *Server) handleCloudflareOAuthZones(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodGet{http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return}
 u:=UserFromContext(r.Context());token,e:=s.cloudflareOAuthAccessToken(r.Context(),u.ID)
 if e!=nil{apiError(w,shared.ErrBadRequest("Cloudflare OAuth is not connected or needs to be reconnected"));return}
 zones,e:=listCloudflareZonesForToken(r.Context(),token);if e!=nil{apiError(w,shared.ErrBadRequest("could not list Cloudflare zones; check that the OAuth client includes zone.read"));return}
 writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"zones":zones})
}

func (s *Server) handleCloudflareOAuthSelectZone(w http.ResponseWriter,r *http.Request){
 if r.Method!=http.MethodPost{http.Error(w,"method not allowed",http.StatusMethodNotAllowed);return}
 u:=UserFromContext(r.Context());var in struct{ZoneID string `json:"zone_id"`}
 if decodeJSON(r,&in)!=nil||!validCloudflareZoneID(strings.TrimSpace(in.ZoneID)){apiError(w,shared.ErrBadRequest("invalid Cloudflare zone ID"));return}
 token,e:=s.cloudflareOAuthAccessToken(r.Context(),u.ID);if e!=nil{apiError(w,shared.ErrBadRequest("Cloudflare OAuth is not connected or needs to be reconnected"));return}
 var zone struct{ID string `json:"id"`;Name string `json:"name"`;Status string `json:"status"`;Account struct{ID string `json:"id"`} `json:"account"`}
 if e=cloudflareAPIRequest(r.Context(),token,http.MethodGet,"/zones/"+in.ZoneID,nil,&zone);e!=nil||zone.ID!=in.ZoneID||zone.Status!="active"||zone.Name==""||!validCloudflareAccountID(zone.Account.ID){apiError(w,shared.ErrBadRequest("Cloudflare zone could not be verified"));return}
 _,e=s.db.conn.ExecContext(r.Context(),`UPDATE cloudflare_accounts SET account_id=?,zone_id=?,zone_name=?,last_verified_at=UTC_TIMESTAMP() WHERE user_id=? AND auth_method='oauth'`,zone.Account.ID,zone.ID,zone.Name,u.ID)
 if e!=nil{apiError(w,shared.ErrInternal("could not save selected Cloudflare zone"));return}
 writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"connected":true,"account_id":zone.Account.ID,"zone_id":zone.ID,"zone_name":zone.Name,"auth_method":"oauth"})
}

func listCloudflareZonesForToken(ctx context.Context,token string)([]map[string]string,error){
 zones:=make([]map[string]string,0)
 for page:=1;page<=10;page++{
  var batch []struct{ID string `json:"id"`;Name string `json:"name"`;Status string `json:"status"`;Account struct{ID string `json:"id"`;Name string `json:"name"`} `json:"account"`}
  if e:=cloudflareAPIRequest(ctx,token,http.MethodGet,"/zones?per_page=50&page="+strconv.Itoa(page),nil,&batch);e!=nil{return nil,e}
  for _,z:=range batch{if z.Status=="active"&&validCloudflareZoneID(z.ID)&&z.Name!=""&&validCloudflareAccountID(z.Account.ID){zones=append(zones,map[string]string{"id":z.ID,"name":z.Name,"account_id":z.Account.ID,"account_name":z.Account.Name})}}
  if len(batch)<50{break}
 }
 return zones,nil
}
