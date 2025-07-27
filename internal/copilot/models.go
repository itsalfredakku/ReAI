package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/devstroop/reai/internal/config"
)

// GetAvailableModels fetches available models dynamically from GitHub Copilot API
func (c *Client) GetAvailableModels(ctx context.Context) ([]ModelInfo, error) {
	slog.Info("GetAvailableModels called - fetching from server")
	
	// Try to fetch models from server
	if models, err := c.fetchModelsFromMultipleSources(ctx); err == nil && len(models) > 0 {
		slog.Info("Successfully fetched models from server", "count", len(models))
		return models, nil
	} else {
		slog.Warn("Failed to fetch models from server", "error", err)
	}

	// No models found - return empty list
	slog.Info("No models found from server - returning empty list")
	return []ModelInfo{}, nil
}

// fetchModelsFromMultipleSources attempts to fetch models from GitHub Copilot endpoints
func (c *Client) fetchModelsFromMultipleSources(ctx context.Context) ([]ModelInfo, error) {
	slog.Info("Starting model fetch from server")
	
	// Get session token
	if !c.isTokenValid() {
		slog.Info("No valid session token, attempting to get one")
		if err := c.GetSessionToken(ctx); err != nil {
			slog.Error("Failed to get session token", "error", err)
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
		slog.Info("Successfully obtained session token")
	} else {
		slog.Info("Using existing valid session token")
	}

	sessionToken := c.sessionToken
	slog.Info("Session token info", "length", len(sessionToken), "prefix", sessionToken[:min(10, len(sessionToken))])

	// Test if our token works with completions endpoint first
	if err := c.testSessionTokenWithCompletions(ctx, sessionToken); err != nil {
		slog.Warn("Session token doesn't work with completions API", "error", err)
		// Don't fail here, just log warning and continue to try models endpoints
	} else {
		slog.Info("Session token is valid - completions API accessible")
	}

	// Try different endpoints that might work
	endpoints := []struct {
		name string
		url  string
	}{
		{"GitHub Copilot API", config.ModelsURL}, // https://api.githubcopilot.com/models
		{"Copilot Proxy Models", config.ModelsURLAlt}, 
		{"GitHub Copilot Individual", "https://api.githubcopilot.com/models"},
		{"GitHub Copilot Business", "https://api.business.githubcopilot.com/models"},
		{"GitHub Copilot Enterprise", "https://api.enterprise.githubcopilot.com/models"},
	}

	for i, endpoint := range endpoints {
		slog.Info("Trying models endpoint", "name", endpoint.name, "url", endpoint.url, "attempt", i+1)
		if models, err := c.tryModelsEndpoint(ctx, sessionToken, endpoint.url); err == nil && len(models) > 0 {
			slog.Info("Successfully fetched models", "source", endpoint.name, "count", len(models))
			return c.deduplicateModels(models), nil
		} else {
			slog.Error("Models endpoint request failed", "name", endpoint.name, "url", endpoint.url, "error", err)
		}
	}

	slog.Error("No models found from any endpoint - server-side only policy")
	// No fallbacks - if server doesn't provide models, return empty
	return []ModelInfo{}, fmt.Errorf("no models available from any server endpoint")
}

// tryModelsEndpoint tries to fetch models from a models endpoint
func (c *Client) tryModelsEndpoint(ctx context.Context, sessionToken, modelsURL string) ([]ModelInfo, error) {
	slog.Debug("Making request to models endpoint", "url", modelsURL)
	
	headers := map[string]string{
		"Authorization":      fmt.Sprintf("Bearer %s", sessionToken),
		"Accept":            "application/json",
		"Content-Type":      "application/json",
		"X-GitHub-Api-Version": "2025-04-01",
	}

	resp, err := c.makeRequest(ctx, "GET", modelsURL, nil, headers)
	if err != nil {
		slog.Error("Models endpoint request failed", "url", modelsURL, "error", err)
		return nil, err
	}

	slog.Debug("Models endpoint response received", "url", modelsURL, "response_length", len(resp))
	
	// Log the actual response for debugging
	if len(resp) < 1000 { // Only log if response is not too large
		slog.Debug("Models endpoint raw response", "url", modelsURL, "response", string(resp))
	}

	return c.parseModelsResponse(resp, modelsURL)
}

// parseModelsResponse attempts to parse model response
func (c *Client) parseModelsResponse(resp []byte, source string) ([]ModelInfo, error) {
	slog.Debug("Parsing models response", "source", source, "response_length", len(resp))
	
	// Try OpenAI-style response
	var modelsResponse struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.Unmarshal(resp, &modelsResponse); err == nil && len(modelsResponse.Data) > 0 {
		slog.Info("Parsed models using OpenAI format", "source", source, "count", len(modelsResponse.Data))
		return modelsResponse.Data, nil
	}

	// Try direct array
	var directModels []ModelInfo
	if err := json.Unmarshal(resp, &directModels); err == nil && len(directModels) > 0 {
		slog.Info("Parsed models using direct array format", "source", source, "count", len(directModels))
		return directModels, nil
	}

	// Try simple names
	var modelNames []string
	if err := json.Unmarshal(resp, &modelNames); err == nil && len(modelNames) > 0 {
		slog.Info("Parsed models using simple names format", "source", source, "count", len(modelNames))
		var models []ModelInfo
		for _, name := range modelNames {
			models = append(models, ModelInfo{
				ID:         name,
				Object:     "model",
				Created:    time.Now().Unix(),
				OwnedBy:    "github",
				Permission: []interface{}{},
				Root:       name,
				Parent:     nil,
			})
		}
		return models, nil
	}

	// Try to parse any JSON and see what we get
	var genericResponse interface{}
	if err := json.Unmarshal(resp, &genericResponse); err == nil {
		slog.Debug("Successfully parsed as JSON", "source", source, "type", fmt.Sprintf("%T", genericResponse))
	} else {
		slog.Error("Failed to parse response as JSON", "source", source, "error", err)
	}

	return nil, fmt.Errorf("unable to parse response from %s", source)
}

func (c *Client) deduplicateModels(models []ModelInfo) []ModelInfo {
	seen := make(map[string]bool)
	var result []ModelInfo
	
	for _, model := range models {
		if !seen[model.ID] {
			seen[model.ID] = true
			result = append(result, model)
		}
	}
	
	return result
}

// testSessionTokenWithCompletions tests if session token works with completions API
func (c *Client) testSessionTokenWithCompletions(ctx context.Context, sessionToken string) error {
	slog.Info("Testing session token with completions API (streaming)")
	
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", sessionToken),
	}

	// Test with streaming=true as the API requires it
	testReq := map[string]interface{}{
		"prompt":      "test",
		"max_tokens":  1,
		"temperature": 0.0,
		"stream":      true, // This is required!
	}

	_, err := c.makeRequest(ctx, "POST", config.CompletionsURL, testReq, headers)
	if err != nil {
		slog.Error("Session token doesn't work with completions API", "error", err)
		return fmt.Errorf("invalid session token: %v", err)
	}
	
	slog.Info("Session token works with completions API")
	return nil
}

// inferBasicModelsFromWorkingAPI infers basic models when completions API works but models API doesn't
func (c *Client) inferBasicModelsFromWorkingAPI(ctx context.Context) ([]ModelInfo, error) {
	slog.Info("All models endpoints failed - returning empty list as per server-side only policy")
	
	// No hardcoded models - if server doesn't provide models, return empty
	return []ModelInfo{}, fmt.Errorf("no models available from server endpoints")
}

// Helper functions
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
