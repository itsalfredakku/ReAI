package copilot

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/devstroop/reai/internal/config"
)

// ModelInfo represents information about an available model
type ModelInfo struct {
	ID         string                 `json:"id"`
	Object     string                 `json:"object"`
	Created    int64                  `json:"created"`
	OwnedBy    string                 `json:"owned_by"`
	Permission []interface{}          `json:"permission"`
	Root       string                 `json:"root"`
	Parent     *string                `json:"parent"`
}

// DeviceCodeResponse represents the response from the device code endpoint
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// AccessTokenResponse represents the response from the access token endpoint
type AccessTokenResponse struct {
	AccessToken *string `json:"access_token,omitempty"`
	Error       *string `json:"error,omitempty"`
}

// SessionTokenResponse represents the response from the session token endpoint
type SessionTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt *int64 `json:"expires_at,omitempty"`
}

// JWTClaims represents JWT token claims
type JWTClaims struct {
	Exp int64                  `json:"exp"`
	//Other map[string]interface{} `json:"-"`
}

// Client represents the GitHub Copilot client
type Client struct {
	config       *config.Config
	httpClient   *http.Client
	accessToken  string
	sessionToken string
	expiresAt    *time.Time
	mutex        sync.RWMutex
}

// NewClient creates a new Copilot client
func NewClient(cfg *config.Config) (*Client, error) {
	client := &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Ensure data directory exists
	if err := client.ensureDataDir(); err != nil {
		slog.Warn("Failed to create data directory", "error", err)
		// Try temporary directory as fallback
		tempDir := filepath.Join(os.TempDir(), "reai")
		cfg.DataDir = tempDir
		if err := client.ensureDataDir(); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
	}

	return client, nil
}

// GetCurrentSessionToken returns the current session token (for debugging only)
func (c *Client) GetCurrentSessionToken() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.sessionToken
}

// ensureDataDir creates the data directory if it doesn't exist
func (c *Client) ensureDataDir() error {
	if err := os.MkdirAll(c.config.DataDir, 0700); err != nil {
		return err
	}
	return nil
}

// Setup performs the GitHub OAuth device flow authentication
func (c *Client) Setup(ctx context.Context) error {
	slog.Info("Starting Copilot authentication setup...")

	// Step 1: Get device code
	deviceReq := map[string]string{
		"client_id": c.config.ClientID,
		"scope":     "read:user",
	}

	deviceResp, err := c.makeRequest(ctx, "POST", config.DeviceCodeURL, deviceReq, nil)
	if err != nil {
		return fmt.Errorf("device code request failed: %w", err)
	}

	var deviceData DeviceCodeResponse
	if err := json.Unmarshal(deviceResp, &deviceData); err != nil {
		return fmt.Errorf("failed to parse device code response: %w", err)
	}

	fmt.Printf("Please visit %s and enter code %s to authenticate.\n", 
		deviceData.VerificationURI, deviceData.UserCode)

	// Step 2: Poll for access token
	ticker := time.NewTicker(time.Duration(deviceData.Interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			tokenReq := map[string]string{
				"client_id":    c.config.ClientID,
				"device_code":  deviceData.DeviceCode,
				"grant_type":   "urn:ietf:params:oauth:grant-type:device_code",
			}

			tokenResp, err := c.makeRequest(ctx, "POST", config.AccessTokenURL, tokenReq, nil)
			if err != nil {
				slog.Warn("Token request failed", "error", err)
				continue
			}

			var tokenData AccessTokenResponse
			if err := json.Unmarshal(tokenResp, &tokenData); err != nil {
				slog.Warn("Failed to parse token response", "error", err)
				continue
			}

			if tokenData.AccessToken != nil {
				c.accessToken = *tokenData.AccessToken
				if err := c.saveAccessToken(*tokenData.AccessToken); err != nil {
					slog.Warn("Failed to save token to file, keeping in memory only", "error", err)
				}
				fmt.Println("Authentication success!")
				return nil
			}

			if tokenData.Error != nil {
				if *tokenData.Error == "authorization_pending" {
					continue
				}
				return fmt.Errorf("authentication error: %s", *tokenData.Error)
			}
		}
	}
}

// saveAccessToken saves the access token to a file
func (c *Client) saveAccessToken(token string) error {
	tokenPath := c.config.TokenFilePath()
	if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
		return err
	}
	return nil
}

// GetSessionToken obtains a session token using the access token
func (c *Client) GetSessionToken(ctx context.Context) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Load access token from file if not in memory
	if c.accessToken == "" {
		tokenPath := c.config.TokenFilePath()
		if data, err := os.ReadFile(tokenPath); err != nil {
			slog.Warn("Failed to load access token from file", "error", err, "path", tokenPath)
			return c.Setup(ctx)
		} else {
			c.accessToken = strings.TrimSpace(string(data))
			slog.Debug("Loaded access token from file")
		}
	}

	// Get session token with retry logic
	for retries := 3; retries > 0; retries-- {
		headers := map[string]string{
			"Authorization": fmt.Sprintf("token %s", c.accessToken),
		}

		resp, err := c.makeRequest(ctx, "GET", config.SessionTokenURL, nil, headers)
		if err != nil {
			return fmt.Errorf("session token request failed: %w", err)
		}

		var tokenData SessionTokenResponse
		if err := json.Unmarshal(resp, &tokenData); err != nil {
			return fmt.Errorf("failed to parse session token response: %w", err)
		}

		// Parse JWT to extract expiration time
		if exp, err := c.extractExpFromJWT(tokenData.Token); err == nil && exp != nil {
			c.expiresAt = exp
		}

		c.sessionToken = tokenData.Token
		slog.Debug("Session token acquired", "expires_at", c.expiresAt)
		return nil
	}

	return fmt.Errorf("failed to get session token after retries")
}

// extractExpFromJWT extracts expiration time from JWT token
func (c *Client) extractExpFromJWT(token string) (*time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		// Not a JWT token, try legacy parsing
		return c.extractExpValueLegacy(token), nil
	}

	// Decode the payload (second part)
	payload := parts[1]
	
	// Add padding if needed for base64 decoding
	padding := 4 - (len(payload) % 4)
	if padding != 4 {
		payload += strings.Repeat("=", padding)
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		slog.Warn("Failed to decode JWT payload, falling back to legacy parsing", "error", err)
		return c.extractExpValueLegacy(token), nil
	}

	var claims JWTClaims
	if err := json.Unmarshal(decoded, &claims); err != nil {
		slog.Warn("Failed to parse JWT claims, falling back to legacy parsing", "error", err)
		return c.extractExpValueLegacy(token), nil
	}

	expTime := time.Unix(claims.Exp, 0)
	return &expTime, nil
}

// extractExpValueLegacy extracts expiration from legacy token format (fallback)
func (c *Client) extractExpValueLegacy(token string) *time.Time {
	for _, pair := range strings.Split(token, ";") {
		if kv := strings.SplitN(pair, "=", 2); len(kv) == 2 {
			if strings.TrimSpace(kv[0]) == "exp" {
				if exp, err := time.Parse(time.RFC3339, strings.TrimSpace(kv[1])); err == nil {
					return &exp
				}
			}
		}
	}
	return nil
}

// isTokenValid checks if the current session token is valid
func (c *Client) isTokenValid() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.sessionToken == "" || c.expiresAt == nil {
		return false
	}

	buffer := time.Duration(config.TokenRefreshBufferSeconds) * time.Second
	return time.Now().Add(buffer).Before(*c.expiresAt)
}

// makeRequest makes an HTTP request with proper headers
func (c *Client) makeRequest(ctx context.Context, method, url string, body interface{}, headers map[string]string) ([]byte, error) {
	var reqBody io.Reader
	
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}

	// Set default headers
	req.Header.Set("User-Agent", config.UserAgent)
	req.Header.Set("Editor-Version", config.EditorVersion)
	req.Header.Set("Editor-Plugin-Version", config.EditorPluginVersion)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2025-04-01")

	// Set custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// StartTokenRefresh starts a background goroutine to refresh tokens
func (c *Client) StartTokenRefresh(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute) // Check every 5 minutes
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !c.isTokenValid() {
				slog.Debug("Token refresh needed")
				if err := c.GetSessionToken(ctx); err != nil {
					slog.Error("Failed to refresh token", "error", err)
				}
			}
		}
	}
}
