package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// GitHub OAuth constants
const (
	ClientID               = "Iv1.b507a08c87ecfe98"
	UserAgent              = "GitHubCopilot/1.228.0"
	EditorVersion          = "vscode/1.87.0"
	EditorPluginVersion    = "copilot/1.228.0"
)

// API endpoints
const (
	DeviceCodeURL    = "https://github.com/login/device/code"
	AccessTokenURL   = "https://github.com/login/oauth/access_token"
	SessionTokenURL  = "https://api.github.com/copilot_internal/v2/token"
	CompletionsURL   = "https://copilot-proxy.githubusercontent.com/v1/engines/copilot-codex/completions"
	ModelsURL        = "https://api.enterprise.githubcopilot.com/models"
	ModelsURLAlt     = "https://api.githubcopilot.com/models"
)

// Token refresh settings
const (
	TokenRefreshBufferSeconds    = 60      // Refresh 60 seconds before expiry
	DefaultTokenLifetimeSeconds  = 25 * 60 // 25 minutes fallback
)

// Rate limiting
const (
	MaxConcurrentRequests = 100
	MaxPromptLength      = 8192
)

// Config holds the application configuration
type Config struct {
	Port             int    `json:"port"`
	ClientID         string `json:"client_id"`
	DataDir          string `json:"data_dir"`
	LogLevel         string `json:"log_level"`
	RateLimit        int    `json:"rate_limit"`
	MaxPromptLength  int    `json:"max_prompt_length"`
}

// LoadFromEnv creates a new Config from environment variables
func LoadFromEnv() *Config {
	port := getEnvInt("PORT", 8080)
	clientID := getEnvString("COPILOT_CLIENT_ID", ClientID)
	
	// Determine data directory with fallback logic
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		xdgDataHome := os.Getenv("XDG_DATA_HOME")
		if xdgDataHome != "" {
			dataDir = filepath.Join(xdgDataHome, "reai")
		} else {
			homeDir := os.Getenv("HOME")
			if homeDir != "" {
				dataDir = filepath.Join(homeDir, ".local", "share", "reai")
			} else {
				dataDir = "/tmp/reai"
			}
		}
	}

	logLevel := getEnvString("LOG_LEVEL", "info")
	rateLimit := getEnvInt("RATE_LIMIT", MaxConcurrentRequests)
	maxPromptLength := getEnvInt("MAX_PROMPT_LENGTH", MaxPromptLength)

	return &Config{
		Port:             port,
		ClientID:         clientID,
		DataDir:          dataDir,
		LogLevel:         logLevel,
		RateLimit:        rateLimit,
		MaxPromptLength:  maxPromptLength,
	}
}

// TokenFilePath returns the path to the token file
func (c *Config) TokenFilePath() string {
	return filepath.Join(c.DataDir, "token")
}

// Helper functions for environment variable handling
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
