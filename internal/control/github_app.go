package control

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type githubAppCredentials struct {
	AppID          string `json:"app_id"`
	InstallationID string `json:"installation_id"`
	PrivateKey     string `json:"private_key"`
}

type githubAppInstallation struct {
	ID      int64 `json:"id"`
	AppID   int64 `json:"app_id"`
	Account struct {
		Login string `json:"login"`
	} `json:"account"`
}

type githubAppAccessToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func validateGitHubAppCredentials(in githubAppCredentials) error {
	appID, appErr := strconv.ParseInt(strings.TrimSpace(in.AppID), 10, 64)
	installationID, installationErr := strconv.ParseInt(strings.TrimSpace(in.InstallationID), 10, 64)
	if appErr != nil || appID < 1 || installationErr != nil || installationID < 1 {
		return fmt.Errorf("GitHub App ID and installation ID must be positive integers")
	}
	if len(in.PrivateKey) < 100 || len(in.PrivateKey) > 16384 || !strings.Contains(in.PrivateKey, "-----BEGIN") || strings.ContainsRune(in.PrivateKey, 0) {
		return fmt.Errorf("a valid PEM private key is required")
	}
	return nil
}

func githubAppJWT(appID, privateKey string, now time.Time) (string, error) {
	block, _ := pem.Decode([]byte(privateKey))
	if block == nil {
		return "", fmt.Errorf("GitHub App private key is not valid PEM")
	}
	var key *rsa.PrivateKey
	if parsed, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		key = parsed
	} else if parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		var ok bool
		key, ok = parsedKey.(*rsa.PrivateKey)
		if !ok { return "", fmt.Errorf("GitHub App key must be RSA") }
	} else {
		return "", fmt.Errorf("could not parse GitHub App RSA private key")
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	appIDValue, err := strconv.ParseInt(appID, 10, 64)
	if err != nil || appIDValue < 1 { return "", fmt.Errorf("invalid GitHub App ID") }
	claims, err := json.Marshal(map[string]interface{}{"iss": appIDValue, "iat": now.Add(-60*time.Second).Unix(), "exp": now.Add(8*time.Minute).Unix()})
	if err != nil { return "", err }
	payload := base64.RawURLEncoding.EncodeToString(claims)
	unsigned := header + "." + payload
	sum := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil { return "", err }
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func githubAppRequest(ctx context.Context, method, endpoint, jwt string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil { return nil, err }
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Yevqo")
	if method == http.MethodPost { req.Header.Set("Content-Type", "application/json") }
	return integrationHTTPClient.Do(req)
}

// githubAppVerificationMessage maps internal failures to actionable, credential-safe diagnostics.
func githubAppVerificationMessage(err error) string {
	if err == nil { return "GitHub App installation verification failed" }
	message := strings.ToLower(err.Error())
	stage := "installation lookup"
	if strings.Contains(message, "create github installation token") {
		stage = "installation token creation"
	}
	switch {
	case strings.Contains(message, "provider returned http 401"):
		return "GitHub rejected the App JWT (HTTP 401). Check that the App ID matches the App that generated the private key."
	case strings.Contains(message, "provider returned http 404"):
		return "GitHub could not find this installation (HTTP 404). Check the installation ID and that the App is installed for this account."
	case strings.Contains(message, "provider returned http 403"):
		return "GitHub denied access to this App installation (HTTP 403). Check the App and installation permissions."
	case strings.Contains(message, "provider returned http 429"):
		return "GitHub API rate limit reached (HTTP 429). Try connecting again later."
	case strings.Contains(message, "provider returned http 422"):
		return "GitHub rejected the " + stage + " request (HTTP 422). Recheck that the App is installed for the selected account and that its installation is active."
	case strings.Contains(message, "provider returned http 400"):
		return "GitHub rejected the " + stage + " request (HTTP 400). Check the App installation configuration."
	case strings.Contains(message, "provider returned http 5"):
		return "GitHub is temporarily failing the " + stage + " request. Try again later."
	case strings.Contains(message, "installation does not belong to this github app"):
		return "The installation belongs to a different GitHub App. Check the configured App ID and installation ID."
	case strings.Contains(message, "not valid pem"), strings.Contains(message, "could not parse github app rsa private key"), strings.Contains(message, "must be rsa"):
		return "The configured GitHub App private key could not be parsed. Upload the private key generated for this App."
	case strings.Contains(message, "x509"), strings.Contains(message, "certificate"), strings.Contains(message, "tls handshake"):
		return "The Control Plane could not establish a trusted TLS connection to GitHub. Check the device date, certificate store and network filtering."
	case strings.Contains(message, "no such host"), strings.Contains(message, "lookup "), strings.Contains(message, "connection refused"), strings.Contains(message, "network is unreachable"), strings.Contains(message, "timeout"), strings.Contains(message, "deadline exceeded"):
		return "The Control Plane could not reach GitHub during " + stage + ". Check DNS, network access and device time, then retry."
	default:
		return "GitHub App verification failed during " + stage + ". Check the App configuration and retry; internal details were withheld."
	}
}

func githubAppInstallationToken(ctx context.Context, credentials githubAppCredentials) (string, string, error) {
	if err := validateGitHubAppCredentials(credentials); err != nil { return "", "", err }
	appID, _ := strconv.ParseInt(credentials.AppID, 10, 64)
	installationID, _ := strconv.ParseInt(credentials.InstallationID, 10, 64)
	jwt, err := githubAppJWT(credentials.AppID, credentials.PrivateKey, time.Now())
	if err != nil { return "", "", err }

	installationURL := fmt.Sprintf("https://api.github.com/app/installations/%d", installationID)
	resp, err := githubAppRequest(ctx, http.MethodGet, installationURL, jwt, nil)
	if err != nil { return "", "", fmt.Errorf("GitHub App installation lookup request failed: %w", err) }
	var installation githubAppInstallation
	if err := readProviderJSON(resp, &installation); err != nil { return "", "", fmt.Errorf("GitHub rejected App installation credentials: %w", err) }
	if installation.ID != installationID || installation.AppID != appID || strings.TrimSpace(installation.Account.Login) == "" {
		return "", "", fmt.Errorf("installation does not belong to this GitHub App")
	}

	tokenURL := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	resp, err = githubAppRequest(ctx, http.MethodPost, tokenURL, jwt, bytes.NewReader([]byte("{}")))
	if err != nil { return "", "", fmt.Errorf("could not create GitHub installation token: request failed: %w", err) }
	var access githubAppAccessToken
	if err := readProviderJSON(resp, &access); err != nil { return "", "", fmt.Errorf("could not create GitHub installation token: %w", err) }
	if access.Token == "" { return "", "", fmt.Errorf("GitHub returned an empty installation token") }
	return access.Token, installation.Account.Login, nil
}

// githubAppInstallationTokenForRepository creates a short-lived installation
// token scoped to one repository. The broader token is used only by the Control
// Plane to resolve the repository ID; it is never sent to an Agent.
func (s *Server) githubAppInstallationTokenForRepository(ctx context.Context, credentials githubAppCredentials, owner, name string) (string, error) {
	installationToken, account, err := githubAppInstallationToken(ctx, credentials)
	if err != nil { return "", err }
	if !strings.EqualFold(account, owner) {
		return "", fmt.Errorf("GitHub App installation owner does not match repository owner")
	}
	repository, err := s.fetchGitHubRepository(ctx, installationToken, owner, name)
	if err != nil { return "", fmt.Errorf("GitHub App installation cannot access repository: %w", err) }
	if repository.ID < 1 { return "", fmt.Errorf("GitHub returned an invalid repository ID") }

	jwt, err := githubAppJWT(credentials.AppID, credentials.PrivateKey, time.Now())
	if err != nil { return "", err }
	body, err := json.Marshal(map[string]interface{}{"repository_ids": []int64{repository.ID}})
	if err != nil { return "", err }
	installationID, err := strconv.ParseInt(credentials.InstallationID, 10, 64)
	if err != nil || installationID < 1 { return "", fmt.Errorf("invalid GitHub App installation ID") }
	endpoint := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	resp, err := githubAppRequest(ctx, http.MethodPost, endpoint, jwt, bytes.NewReader(body))
	if err != nil { return "", fmt.Errorf("could not create repository-scoped GitHub token: %w", err) }
	var access githubAppAccessToken
	if err := readProviderJSON(resp, &access); err != nil { return "", fmt.Errorf("could not create repository-scoped GitHub token: %w", err) }
	if strings.TrimSpace(access.Token) == "" { return "", fmt.Errorf("GitHub returned an empty repository-scoped token") }
	return access.Token, nil
}

// githubAppTokenForIntegrationRepository loads one user's installation and
// issues a token limited to the exact repository linked to the site/project.
func (s *Server) githubAppTokenForIntegrationRepository(ctx context.Context, userID, integrationID int64, fullName string) (string, error) {
	owner, name, _, err := parseGitHubRepositoryName(fullName)
	if err != nil { return "", err }
	var nonce, ciphertext []byte
	var account string
	if err := s.db.conn.QueryRowContext(ctx, `SELECT account_name,token_nonce,token_ciphertext FROM integrations WHERE id=? AND user_id=? AND provider='github_app'`, integrationID, userID).Scan(&account, &nonce, &ciphertext); err != nil {
		return "", fmt.Errorf("GitHub App installation not found")
	}
	if !strings.EqualFold(account, owner) { return "", fmt.Errorf("GitHub App installation owner mismatch") }
	raw, err := decryptCredential(s.credentialKey, nonce, ciphertext)
	if err != nil { return "", fmt.Errorf("could not decrypt GitHub App credentials") }
	var credentials githubAppCredentials
	if err := json.Unmarshal(raw, &credentials); err != nil { return "", fmt.Errorf("stored GitHub App credentials are invalid") }
	return s.githubAppInstallationTokenForRepository(ctx, credentials, owner, name)
}

func (s *Server) githubAppTokenForRepository(ctx context.Context, userID int64, repositoryURL string) (string, error) {
	parsed, err := url.Parse(repositoryURL)
	if err != nil || !strings.EqualFold(parsed.Hostname(), "github.com") || parsed.Scheme != "https" { return "", nil }
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 { return "", nil }
	fullName := strings.TrimSuffix(parts[0]+"/"+parts[1], ".git")
	owner := parts[0]

	rows, err := s.db.conn.QueryContext(ctx, `SELECT id,account_name FROM integrations WHERE user_id=? AND provider='github_app' ORDER BY connected_at DESC`, userID)
	if err != nil { return "", err }
	defer rows.Close()
	for rows.Next() {
		var integrationID int64
		var account string
		if err := rows.Scan(&integrationID, &account); err != nil { return "", err }
		if !strings.EqualFold(account, owner) { continue }
		return s.githubAppTokenForIntegrationRepository(ctx, userID, integrationID, fullName)
	}
	if err := rows.Err(); err != nil { return "", err }
	return "", nil
}
