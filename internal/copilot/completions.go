package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/devstroop/reai/internal/config"
	"github.com/devstroop/reai/pkg/errors"
)

// CompletionRequest represents a completion request
type CompletionRequest struct {
	Prompt      string `json:"prompt"`
	Language    string `json:"language,omitempty"`
	MaxTokens   int    `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	Stream      bool   `json:"stream,omitempty"`
}

// GetCompletion gets a code completion from GitHub Copilot
func (c *Client) GetCompletion(ctx context.Context, req *CompletionRequest) (string, error) {
	// Validate prompt length
	if len(req.Prompt) > c.config.MaxPromptLength {
		return "", errors.NewValidationError(fmt.Sprintf("Prompt too long: %d characters (max: %d)", 
			len(req.Prompt), c.config.MaxPromptLength))
	}

	// Ensure we have a valid token
	if !c.isTokenValid() {
		if err := c.GetSessionToken(ctx); err != nil {
			return "", errors.NewAuthenticationError(err.Error())
		}
	}

	sessionToken := c.sessionToken
	if sessionToken == "" {
		return "", errors.NewAuthenticationError("No session token available")
	}

	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", sessionToken),
	}

	// Set defaults
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1000
	}
	
	temperature := req.Temperature
	if temperature == 0 {
		temperature = 0.0
	}

	language := req.Language
	if language == "" {
		language = "text"
	}

	copilotReq := map[string]interface{}{
		"prompt":      req.Prompt,
		"suffix":      "",
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"top_p":       1,
		"n":          1,
		"stop":       []string{"\n"},
		"nwo":        "github/copilot.vim",
		"stream":     true,
		"extra": map[string]interface{}{
			"language": language,
		},
	}

	resp, err := c.makeRequest(ctx, "POST", config.CompletionsURL, copilotReq, headers)
	if err != nil {
		return "", errors.NewCopilotAPIError(fmt.Sprintf("Completion request failed: %s", err.Error()))
	}

	return c.parseStreamingResponse(string(resp))
}

// parseStreamingResponse parses the streaming response from Copilot
func (c *Client) parseStreamingResponse(responseText string) (string, error) {
	var result strings.Builder

	for _, line := range strings.Split(responseText, "\n") {
		if strings.HasPrefix(line, "data: {") {
			jsonData := line[6:] // Remove "data: " prefix
			
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
				slog.Debug("Failed to parse streaming chunk", "error", err, "data", jsonData)
				continue
			}

			if choices, ok := data["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if text, ok := choice["text"].(string); ok {
						result.WriteString(text)
					}
				}
			}
		}
	}

	return result.String(), nil
}
