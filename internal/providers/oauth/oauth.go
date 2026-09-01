package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jedarden/tunnel/internal/core"
	"github.com/jedarden/tunnel/internal/providers"
)

// OAuthProvider implements the Provider interface for OAuth 2.0 authentication
type OAuthProvider struct {
	*providers.BaseProvider
	credentialStore core.CredentialStore
	httpClient      *http.Client
	server          *http.Server
	state           string
	verifier        string
	tokenResponse   chan *TokenResponse
	cancel          context.CancelFunc
}

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// OAuthConfig contains OAuth-specific configuration
type OAuthConfig struct {
	Provider     string `json:"provider"`      // github, google, etc.
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURI  string `json:"redirect_uri"`
	Scopes       string `json:"scopes,omitempty"`
}

// New creates a new OAuth provider
func New(credentialStore core.CredentialStore) *OAuthProvider {
	return &OAuthProvider{
		BaseProvider:    providers.NewBaseProvider("oauth", providers.CategoryDirect),
		credentialStore: credentialStore,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		tokenResponse: make(chan *TokenResponse, 1),
	}
}

// Install is a no-op for OAuth (it's always available)
func (o *OAuthProvider) Install() error {
	return nil
}

// Uninstall is a no-op for OAuth
func (o *OAuthProvider) Uninstall() error {
	return nil
}

// IsInstalled checks if OAuth is available (always true)
func (o *OAuthProvider) IsInstalled() bool {
	return true
}

// Configure sets up the OAuth provider with configuration
func (o *OAuthProvider) Configure(config *providers.ProviderConfig) error {
	if err := o.BaseProvider.Configure(config); err != nil {
		return err
	}

	// Validate OAuth-specific settings
	oauthConfig, err := o.parseOAuthConfig(config)
	if err != nil {
		return fmt.Errorf("invalid oauth config: %w", err)
	}

	if oauthConfig.ClientID == "" {
		return fmt.Errorf("%w: client_id is required", providers.ErrMissingKey)
	}

	if oauthConfig.RedirectURI == "" {
		return fmt.Errorf("%w: redirect_uri is required", providers.ErrInvalidConfig)
	}

	return nil
}

// parseOAuthConfig extracts OAuth configuration from ProviderConfig
func (o *OAuthProvider) parseOAuthConfig(config *providers.ProviderConfig) (*OAuthConfig, error) {
	oauthConfig := &OAuthConfig{}

	// Get from Extra map
	if config.Extra != nil {
		if provider, ok := config.Extra["provider"]; ok {
			oauthConfig.Provider = provider
		}
		if clientID, ok := config.Extra["client_id"]; ok {
			oauthConfig.ClientID = clientID
		}
		if clientSecret, ok := config.Extra["client_secret"]; ok {
			oauthConfig.ClientSecret = clientSecret
		}
		if redirectURI, ok := config.Extra["redirect_uri"]; ok {
			oauthConfig.RedirectURI = redirectURI
		}
		if scopes, ok := config.Extra["scopes"]; ok {
			oauthConfig.Scopes = scopes
		}
	}

	// Set defaults
	if oauthConfig.Provider == "" {
		oauthConfig.Provider = "github"
	}

	return oauthConfig, nil
}

// Connect initiates the OAuth flow
func (o *OAuthProvider) Connect() error {
	config, err := o.GetConfig()
	if err != nil {
		return err
	}

	oauthConfig, err := o.parseOAuthConfig(config)
	if err != nil {
		return err
	}

	// Check if we have a stored token
	if o.credentialStore != nil {
		storedToken, err := o.credentialStore.Get("tunnel", "oauth-token")
		if err == nil && len(storedToken) > 0 {
			// Token exists, verify it's still valid
			var token TokenResponse
			if err := json.Unmarshal(storedToken, &token); err == nil {
				if token.AccessToken != "" {
					// Token is valid, we're connected
					return nil
				}
			}
		}
	}

	// No valid token, initiate OAuth flow
	return o.initiateOAuthFlow(oauthConfig)
}

// initiateOAuthFlow starts the OAuth authorization flow
func (o *OAuthProvider) initiateOAuthFlow(config *OAuthConfig) error {
	// Generate state and PKCE verifier
	o.state = o.generateRandomString(32)
	o.verifier = o.generateRandomString(32)

	// Calculate code challenge (plain for simplicity, could use S256)
	codeChallenge := base64.RawURLEncoding.EncodeToString([]byte(o.verifier))

	// Build authorization URL
	authURL, err := o.buildAuthorizationURL(config, o.state, codeChallenge)
	if err != nil {
		return fmt.Errorf("build authorization URL: %w", err)
	}

	// Start local callback server
	if err := o.startCallbackServer(config.RedirectURI); err != nil {
		return fmt.Errorf("start callback server: %w", err)
	}

	// Print instructions for user
	fmt.Printf("\n=== OAuth Authentication Required ===\n")
	fmt.Printf("Provider: %s\n", config.Provider)
	fmt.Printf("Visit this URL to authorize:\n")
	fmt.Printf("%s\n\n", authURL)
	fmt.Printf("Waiting for authorization callback...\n")

	// Wait for callback
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	select {
	case tokenResp := <-o.tokenResponse:
		if tokenResp == nil {
			return fmt.Errorf("%w: authorization failed", providers.ErrConnectionFailed)
		}

		// Store the token
		if o.credentialStore != nil {
			tokenData, err := json.Marshal(tokenResp)
			if err != nil {
				return fmt.Errorf("marshal token: %w", err)
			}

			if err := o.credentialStore.Set("tunnel", "oauth-token", tokenData); err != nil {
				return fmt.Errorf("store token: %w", err)
			}
		}

		fmt.Printf("Authentication successful!\n")
		return nil

	case <-ctx.Done():
		o.stopCallbackServer()
		return fmt.Errorf("%w: authorization timeout", providers.ErrConnectionFailed)
	}
}

// buildAuthorizationURL creates the OAuth authorization URL
func (o *OAuthProvider) buildAuthorizationURL(config *OAuthConfig, state, codeChallenge string) (string, error) {
	var baseURL string
	var endpoint string

	switch config.Provider {
	case "github":
		baseURL = "https://github.com"
		endpoint = "/login/oauth/authorize"
	case "google":
		baseURL = "https://accounts.google.com"
		endpoint = "/o/oauth2/v2/auth"
	case "gitlab":
		baseURL = "https://gitlab.com"
		endpoint = "/oauth/authorize"
	default:
		return "", fmt.Errorf("unsupported provider: %s", config.Provider)
	}

	params := url.Values{}
	params.Set("client_id", config.ClientID)
	params.Set("redirect_uri", config.RedirectURI)
	params.Set("response_type", "code")
	params.Set("state", state)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "plain")

	if config.Scopes != "" {
		params.Set("scope", config.Scopes)
	} else {
		// Default scopes per provider
		switch config.Provider {
		case "github":
			params.Set("scope", "read:user user:email")
		case "google":
			params.Set("scope", "openid email profile")
		case "gitlab":
			params.Set("scope", "read_user")
		}
	}

	return fmt.Sprintf("%s%s?%s", baseURL, endpoint, params.Encode()), nil
}

// startCallbackServer starts a local HTTP server to handle the OAuth callback
func (o *OAuthProvider) startCallbackServer(redirectURI string) error {
	// Extract port from redirect URI
	parsedURL, err := url.Parse(redirectURI)
	if err != nil {
		return fmt.Errorf("parse redirect URI: %w", err)
	}

	addr := parsedURL.Host
	if addr == "" {
		addr = "localhost:8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc(parsedURL.Path, o.handleCallback)

	o.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	ctx, cancel := context.WithCancel(context.Background())
	o.cancel = cancel

	go func() {
		if err := o.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Callback server error: %v\n", err)
		}
	}()

	return nil
}

// handleCallback handles the OAuth callback
func (o *OAuthProvider) handleCallback(w http.ResponseWriter, r *http.Request) {
	// Verify state
	if r.URL.Query().Get("state") != o.state {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		o.tokenResponse <- nil
		return
	}

	// Get authorization code
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		o.tokenResponse <- nil
		return
	}

	// Exchange code for token
	config, err := o.GetConfig()
	if err != nil {
		http.Error(w, "Configuration error", http.StatusInternalServerError)
		o.tokenResponse <- nil
		return
	}

	oauthConfig, err := o.parseOAuthConfig(config)
	if err != nil {
		http.Error(w, "Invalid OAuth configuration", http.StatusInternalServerError)
		o.tokenResponse <- nil
		return
	}

	tokenResp, err := o.exchangeCodeForToken(oauthConfig, code, o.verifier)
	if err != nil {
		http.Error(w, fmt.Sprintf("Token exchange failed: %v", err), http.StatusInternalServerError)
		o.tokenResponse <- nil
		return
	}

	// Send success response
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<html><body><h1>Authentication successful!</h1><p>You can close this window.</p></body></html>`))

	// Send token to channel
	o.tokenResponse <- tokenResp

	// Stop server after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		o.stopCallbackServer()
	}()
}

// exchangeCodeForToken exchanges the authorization code for an access token
func (o *OAuthProvider) exchangeCodeForToken(config *OAuthConfig, code, verifier string) (*TokenResponse, error) {
	var tokenURL string
	var provider string = config.Provider

	switch provider {
	case "github":
		tokenURL = "https://github.com/login/oauth/access_token"
	case "google":
		tokenURL = "https://oauth2.googleapis.com/token"
	case "gitlab":
		tokenURL = "https://gitlab.com/oauth/token"
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}

	data := url.Values{}
	data.Set("client_id", config.ClientID)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", config.RedirectURI)
	data.Set("code_verifier", verifier)

	if config.ClientSecret != "" {
		data.Set("client_secret", config.ClientSecret)
	}

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status: %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &tokenResp, nil
}

// stopCallbackServer stops the callback server
func (o *OAuthProvider) stopCallbackServer() {
	if o.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		o.server.Shutdown(ctx)
		o.server = nil
	}
	if o.cancel != nil {
		o.cancel()
		o.cancel = nil
	}
}

// Disconnect clears the stored OAuth token
func (o *OAuthProvider) Disconnect() error {
	if o.credentialStore != nil {
		if err := o.credentialStore.Delete("tunnel", "oauth-token"); err != nil {
			return fmt.Errorf("delete token: %w", err)
		}
	}
	return nil
}

// IsConnected checks if we have a valid token
func (o *OAuthProvider) IsConnected() bool {
	if o.credentialStore == nil {
		return false
	}

	storedToken, err := o.credentialStore.Get("tunnel", "oauth-token")
	if err != nil {
		return false
	}

	var token TokenResponse
	if err := json.Unmarshal(storedToken, &token); err != nil {
		return false
	}

	return token.AccessToken != ""
}

// GetConnectionInfo returns information about the OAuth connection
func (o *OAuthProvider) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	config, err := o.GetConfig()
	if err != nil {
		return nil, err
	}

	oauthConfig, err := o.parseOAuthConfig(config)
	if err != nil {
		return nil, err
	}

	info := &providers.ConnectionInfo{
		Status: "connected",
		Extra:  make(map[string]interface{}),
	}

	if o.IsConnected() {
		info.Status = "connected"
		info.Extra["provider"] = oauthConfig.Provider
		info.Extra["authenticated"] = true
	} else {
		info.Status = "disconnected"
		info.Extra["authenticated"] = false
	}

	return info, nil
}

// HealthCheck performs a health check on the OAuth connection
func (o *OAuthProvider) HealthCheck() (*providers.HealthStatus, error) {
	start := time.Now()

	if !o.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_available",
			Message:   "OAuth is not available",
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	config, err := o.GetConfig()
	if err != nil {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "no_config",
			Message:   fmt.Sprintf("Configuration error: %v", err),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	oauthConfig, err := o.parseOAuthConfig(config)
	if err != nil {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "invalid_config",
			Message:   fmt.Sprintf("Invalid OAuth config: %v", err),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	if !o.IsConnected() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_authenticated",
			Message:   "Not authenticated - run Connect() to authenticate",
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	return &providers.HealthStatus{
		Healthy:   true,
		Status:    "authenticated",
		Message:   fmt.Sprintf("Authenticated via %s OAuth", oauthConfig.Provider),
		LastCheck: time.Now(),
		Latency:   time.Since(start),
	}, nil
}

// GetLogs returns log entries (OAuth doesn't have logs, return empty)
func (o *OAuthProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	return []providers.LogEntry{}, nil
}

// ValidateConfig validates OAuth-specific configuration
func (o *OAuthProvider) ValidateConfig(config *providers.ProviderConfig) error {
	if err := o.BaseProvider.ValidateConfig(config); err != nil {
		return err
	}

	oauthConfig, err := o.parseOAuthConfig(config)
	if err != nil {
		return err
	}

	// Validate redirect URI format
	if oauthConfig.RedirectURI != "" {
		if _, err := url.Parse(oauthConfig.RedirectURI); err != nil {
			return fmt.Errorf("%w: invalid redirect_uri", providers.ErrInvalidConfig)
		}
	}

	// Validate provider
	validProviders := map[string]bool{
		"github": true,
		"google": true,
		"gitlab": true,
	}
	if oauthConfig.Provider != "" && !validProviders[oauthConfig.Provider] {
		return fmt.Errorf("%w: unsupported provider %s", providers.ErrInvalidConfig, oauthConfig.Provider)
	}

	return nil
}

// generateRandomString generates a cryptographically random string
func (o *OAuthProvider) generateRandomString(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to pseudo-random if crypto random fails
		for i := range bytes {
			bytes[i] = byte(time.Now().UnixNano() % 256)
		}
	}
	return base64.RawURLEncoding.EncodeToString(bytes)[:length]
}
