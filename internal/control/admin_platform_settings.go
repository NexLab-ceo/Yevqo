package control

import (
    "database/sql"
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
    if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "https" && !(parsed.Scheme == "http" && (parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1"))) {
        return "", shared.ErrBadRequest("Public URL must be an HTTPS origin without a path, query or fragment")
    }
    return raw, nil
}

func (s *Server) handleAdminPlatformSettings(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodGet:
        var publicURL, clientID string
        var nonce, ciphertext []byte
        err := s.db.conn.QueryRowContext(r.Context(), `SELECT public_url,cloudflare_oauth_client_id,cloudflare_oauth_secret_nonce,cloudflare_oauth_secret_ciphertext FROM platform_settings WHERE id=1`).Scan(&publicURL,&clientID,&nonce,&ciphertext)
        if err != nil && err != sql.ErrNoRows { apiError(w, shared.ErrInternal("could not load platform settings")); return }
        if publicURL == "" { publicURL = strings.TrimRight(strings.TrimSpace(os.Getenv("YEVQO_PUBLIC_URL")), "/") }
        if clientID == "" { clientID = strings.TrimSpace(os.Getenv("YEVQO_CLOUDFLARE_OAUTH_CLIENT_ID")) }
        base, _ := normalizePlatformURL(publicURL)
        callback := ""
        if base != "" { callback = base + "/oauth/cloudflare/callback" }
        writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"public_url":publicURL,"callback_url":callback,"cloudflare_oauth_client_id":clientID,"cloudflare_oauth_secret_set":len(nonce)>0 && len(ciphertext)>0,"github_app_configured":strings.TrimSpace(os.Getenv("YEVQO_GITHUB_APP_ID"))!="" && strings.TrimSpace(os.Getenv("YEVQO_GITHUB_APP_PRIVATE_KEY"))!=""})
    case http.MethodPut:
        var in struct {
            PublicURL string `json:"public_url"`
            ClientID string `json:"cloudflare_oauth_client_id"`
            ClientSecret string `json:"cloudflare_oauth_client_secret"`
            ClearSecret bool `json:"clear_cloudflare_oauth_secret"`
        }
        if err:=decodeJSON(r,&in); err!=nil { apiError(w,shared.ErrBadRequest("invalid platform settings")); return }
        publicURL,err:=normalizePlatformURL(in.PublicURL); if err!=nil { apiError(w,err); return }
        in.ClientID=strings.TrimSpace(in.ClientID)
        if len(in.ClientID)>255 || strings.ContainsAny(in.ClientID,"\\r\\n\\x00") { apiError(w,shared.ErrBadRequest("invalid Cloudflare OAuth Client ID")); return }
        if len(in.ClientSecret)>4096 || strings.ContainsAny(in.ClientSecret,"\\r\\n\\x00") { apiError(w,shared.ErrBadRequest("invalid Cloudflare OAuth Client Secret")); return }
        if in.ClientSecret!="" && in.ClientID=="" { apiError(w,shared.ErrBadRequest("Client ID is required when setting a Client Secret")); return }
        var nonce,ciphertext []byte
        if in.ClientSecret!="" {
            nonce,ciphertext,err=encryptCredential(s.credentialKey,[]byte(in.ClientSecret))
            if err!=nil { apiError(w,shared.ErrInternal("could not encrypt OAuth Client Secret")); return }
        } else if !in.ClearSecret {
            _ = s.db.conn.QueryRowContext(r.Context(), `SELECT cloudflare_oauth_secret_nonce,cloudflare_oauth_secret_ciphertext FROM platform_settings WHERE id=1`).Scan(&nonce,&ciphertext)
        }
        if in.ClearSecret { nonce=nil; ciphertext=nil }
        _,err=s.db.conn.ExecContext(r.Context(), `INSERT INTO platform_settings(id,public_url,cloudflare_oauth_client_id,cloudflare_oauth_secret_nonce,cloudflare_oauth_secret_ciphertext) VALUES(1,?,?,?,?) ON DUPLICATE KEY UPDATE public_url=VALUES(public_url),cloudflare_oauth_client_id=VALUES(cloudflare_oauth_client_id),cloudflare_oauth_secret_nonce=VALUES(cloudflare_oauth_secret_nonce),cloudflare_oauth_secret_ciphertext=VALUES(cloudflare_oauth_secret_ciphertext)`,publicURL,in.ClientID,nullableBytes(nonce),nullableBytes(ciphertext))
        if err!=nil { apiError(w,shared.ErrInternal("could not save platform settings")); return }
        callback:=publicURL+"/oauth/cloudflare/callback"
        writeJSON(w,http.StatusOK,map[string]interface{}{"ok":true,"public_url":publicURL,"callback_url":callback,"cloudflare_oauth_client_id":in.ClientID,"cloudflare_oauth_secret_set":len(nonce)>0 && len(ciphertext)>0})
    default:
        http.Error(w,"method not allowed",http.StatusMethodNotAllowed)
    }
}

func nullableBytes(value []byte) interface{} {
    if len(value)==0 { return nil }
    return value
}
