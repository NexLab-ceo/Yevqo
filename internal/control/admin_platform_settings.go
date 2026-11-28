package control

import (
    "crypto/rsa"
    "crypto/x509"
    "database/sql"
    "encoding/pem"
    "strconv"
    "net/http"
    "net/url"
    "os"
    "strings"

    "veltrix/internal/shared"
)

func normalizePlatformURL(raw string) (string, error) {
    raw = strings.TrimRight(strings.TrimSpace(raw), "/")
    if len(raw) > 512 { return "", shared.ErrBadRequest("Public URL is too long") }
    parsed, err := url.Parse(raw)
    if err != nil || parsed.Host == "" || parsed.Path != "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "https" && !(parsed.Scheme == "http" && (parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1"))) {
        return "", shared.ErrBadRequest("Public URL must be an HTTPS origin without a path, query or fragment")
    }
    return raw, nil
}

func (s *Server) handleAdminPlatformSettings(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        var publicURL, clientID, githubAppID, githubAppSlug string
        var nonce, ciphertext, githubNonce, githubCiphertext []byte
        err := s.db.conn.QueryRowContext(r.Context(), `SELECT public_url,cloudflare_oauth_client_id,cloudflare_oauth_secret_nonce,cloudflare_oauth_secret_ciphertext,github_app_id,github_app_slug,github_app_private_key_nonce,github_app_private_key_ciphertext FROM platform_settings WHERE id=1`).Scan(&publicURL,&clientID,&nonce,&ciphertext,&githubAppID,&githubAppSlug,&githubNonce,&githubCiphertext)
        if err != nil && err != sql.ErrNoRows { apiError(w, shared.ErrInternal("could not load platform settings")); return }
        if publicURL == "" { publicURL = strings.TrimRight(strings.TrimSpace(os.Getenv("YEVQO_PUBLIC_URL")), "/") }
        if clientID == "" { clientID = strings.TrimSpace(os.Getenv("YEVQO_CLOUDFLARE_OAUTH_CLIENT_ID")) }
        if githubAppID == "" { githubAppID = strings.TrimSpace(os.Getenv("YEVQO_GITHUB_APP_ID")) }
        if githubAppSlug == "" { githubAppSlug = strings.TrimSpace(os.Getenv("YEVQO_GITHUB_APP_SLUG")) }
        githubKeySet := len(githubNonce)>0 && len(githubCiphertext)>0
        if !githubKeySet { githubKeySet = strings.TrimSpace(os.Getenv("YEVQO_GITHUB_APP_PRIVATE_KEY")) != "" }
        base, _ := normalizePlatformURL(publicURL)
        callback := ""
        if base != "" { callback = base + "/oauth/cloudflare/callback" }
        writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"public_url":publicURL,"callback_url":callback,"cloudflare_oauth_client_id":clientID,"cloudflare_oauth_secret_set":len(nonce)>0 && len(ciphertext)>0,"github_app_id":githubAppID,"github_app_slug":githubAppSlug,"github_app_private_key_set":githubKeySet,"github_app_configured":githubAppID!="" && githubAppSlug!="" && githubKeySet})
    case http.MethodPut:
        var in struct {
            PublicURL string `json:"public_url"`
            ClientID string `json:"cloudflare_oauth_client_id"`
            ClientSecret string `json:"cloudflare_oauth_client_secret"`
            ClearSecret bool `json:"clear_cloudflare_oauth_secret"`
            GitHubAppID string `json:"github_app_id"`
            GitHubAppSlug string `json:"github_app_slug"`
            GitHubPrivateKey string `json:"github_app_private_key"`
            ClearGitHubPrivateKey bool `json:"clear_github_app_private_key"`
        }
        if err:=decodeJSON(r,&in); err!=nil { apiError(w,shared.ErrBadRequest("invalid platform settings")); return }
        publicURL,err:=normalizePlatformURL(in.PublicURL); if err!=nil { apiError(w,err); return }
        in.ClientID=strings.TrimSpace(in.ClientID)
        if len(in.ClientID)>255 || strings.ContainsAny(in.ClientID,"\r\n\x00") { apiError(w,shared.ErrBadRequest("invalid Cloudflare OAuth Client ID")); return }
        if len(in.ClientSecret)>4096 || strings.ContainsAny(in.ClientSecret,"\r\n\x00") { apiError(w,shared.ErrBadRequest("invalid Cloudflare OAuth Client Secret")); return }
        in.GitHubAppID = strings.TrimSpace(in.GitHubAppID)
        in.GitHubAppSlug = strings.TrimSpace(in.GitHubAppSlug)
        if in.GitHubAppID != "" { id, parseErr := strconv.ParseInt(in.GitHubAppID,10,64); if parseErr != nil || id < 1 { apiError(w,shared.ErrBadRequest("GitHub App ID must be a positive integer")); return } }
        if len(in.GitHubAppSlug)>255 || strings.ContainsAny(in.GitHubAppSlug,"/\\?# \r\n\x00") { apiError(w,shared.ErrBadRequest("invalid GitHub App slug")); return }
        in.GitHubPrivateKey = strings.TrimSpace(strings.ReplaceAll(in.GitHubPrivateKey,"\\n","\n"))
        if len(in.GitHubPrivateKey)>16384 || strings.ContainsRune(in.GitHubPrivateKey,0) { apiError(w,shared.ErrBadRequest("invalid GitHub App private key")); return }
        if in.GitHubPrivateKey != "" {
            block,_ := pem.Decode([]byte(in.GitHubPrivateKey))
            if block == nil { apiError(w,shared.ErrBadRequest("GitHub App private key must be a PEM RSA key")); return }
            if _, parseErr := x509.ParsePKCS1PrivateKey(block.Bytes); parseErr != nil {
                parsed, pkcs8Err := x509.ParsePKCS8PrivateKey(block.Bytes)
                if pkcs8Err != nil { apiError(w,shared.ErrBadRequest("GitHub App private key could not be parsed")); return }
                if _, ok := parsed.(*rsa.PrivateKey); !ok { apiError(w,shared.ErrBadRequest("GitHub App private key must be RSA")); return }
            }
        }
        if (in.GitHubAppID != "" || in.GitHubAppSlug != "" || in.GitHubPrivateKey != "") && (in.GitHubAppID == "" || in.GitHubAppSlug == "") { apiError(w,shared.ErrBadRequest("GitHub App ID and slug are required together")); return }
        if in.ClientSecret!="" && in.ClientID=="" { apiError(w,shared.ErrBadRequest("Client ID is required when setting a Client Secret")); return }
        var nonce,ciphertext,githubNonce,githubCiphertext []byte
        if in.ClientSecret!="" {
            nonce,ciphertext,err=encryptCredential(s.credentialKey,[]byte(in.ClientSecret))
            if err!=nil { apiError(w,shared.ErrInternal("could not encrypt OAuth Client Secret")); return }
        } else if !in.ClearSecret {
            _ = s.db.conn.QueryRowContext(r.Context(), `SELECT cloudflare_oauth_secret_nonce,cloudflare_oauth_secret_ciphertext FROM platform_settings WHERE id=1`).Scan(&nonce,&ciphertext)
        }
        if in.ClearSecret { nonce=nil; ciphertext=nil }
        if in.GitHubPrivateKey != "" {
            githubNonce,githubCiphertext,err = encryptCredential(s.credentialKey,[]byte(in.GitHubPrivateKey))
            if err != nil { apiError(w,shared.ErrInternal("could not encrypt GitHub App private key")); return }
        } else if !in.ClearGitHubPrivateKey {
            _ = s.db.conn.QueryRowContext(r.Context(), `SELECT github_app_private_key_nonce,github_app_private_key_ciphertext FROM platform_settings WHERE id=1`).Scan(&githubNonce,&githubCiphertext)
        }
        if in.ClearGitHubPrivateKey { githubNonce=nil; githubCiphertext=nil }
        _,err=s.db.conn.ExecContext(r.Context(), `INSERT INTO platform_settings(id,public_url,cloudflare_oauth_client_id,cloudflare_oauth_secret_nonce,cloudflare_oauth_secret_ciphertext,github_app_id,github_app_slug,github_app_private_key_nonce,github_app_private_key_ciphertext) VALUES(1,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE public_url=VALUES(public_url),cloudflare_oauth_client_id=VALUES(cloudflare_oauth_client_id),cloudflare_oauth_secret_nonce=VALUES(cloudflare_oauth_secret_nonce),cloudflare_oauth_secret_ciphertext=VALUES(cloudflare_oauth_secret_ciphertext),github_app_id=VALUES(github_app_id),github_app_slug=VALUES(github_app_slug),github_app_private_key_nonce=VALUES(github_app_private_key_nonce),github_app_private_key_ciphertext=VALUES(github_app_private_key_ciphertext)`,publicURL,in.ClientID,nullableBytes(nonce),nullableBytes(ciphertext),in.GitHubAppID,in.GitHubAppSlug,nullableBytes(githubNonce),nullableBytes(githubCiphertext))
        if err!=nil { apiError(w,shared.ErrInternal("could not save platform settings")); return }
        callback:=publicURL+"/oauth/cloudflare/callback"
        writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"public_url":publicURL,"callback_url":callback,"cloudflare_oauth_client_id":in.ClientID,"cloudflare_oauth_secret_set":len(nonce)>0 && len(ciphertext)>0,"github_app_id":in.GitHubAppID,"github_app_slug":in.GitHubAppSlug,"github_app_private_key_set":len(githubNonce)>0 && len(githubCiphertext)>0,"github_app_configured":in.GitHubAppID!="" && in.GitHubAppSlug!="" && len(githubNonce)>0 && len(githubCiphertext)>0})
    default:
        http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
    }
}

func nullableBytes(value []byte) interface{} {
    if len(value)==0 { return nil }
    return value
}
