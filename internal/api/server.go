package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/devstroop/reai/internal/copilot"
	"github.com/devstroop/reai/pkg/errors"
)

// Server represents the API server
type Server struct {
	copilotClient *copilot.Client
}

// NewServer creates a new API server
func NewServer(client *copilot.Client) *Server {
	return &Server{
		copilotClient: client,
	}
}

// Router returns the HTTP router for the server
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", s.handleHealth)
	
	// Debug endpoint to get token (for testing only)
	mux.HandleFunc("/debug/token", s.handleDebugToken)
	
	// Models endpoint
	mux.HandleFunc("/v1/models", s.handleModels)
	
	// Completions endpoint
	mux.HandleFunc("/v1/completions", s.handleCompletions)
	
	// Chat completions endpoint (basic implementation)
	mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)

	// Add middleware
	return s.loggingMiddleware(s.corsMiddleware(mux))
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"service":   "reai",
		"version":   "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDebugToken handles debug token requests (for testing only)
func (s *Server) handleDebugToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get session token from client
	ctx := r.Context()
	if err := s.copilotClient.GetSessionToken(ctx); err != nil {
		http.Error(w, "Failed to get session token", http.StatusInternalServerError)
		return
	}

	// Return the token for manual testing
	response := map[string]interface{}{
		"session_token": s.copilotClient.GetCurrentSessionToken(),
		"warning": "This is for testing only - do not expose in production",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleModels handles model listing requests
func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	
	models, err := s.copilotClient.GetAvailableModels(ctx)
	if err != nil {
		slog.Error("Failed to fetch models", "error", err)
		errors.WriteErrorResponse(w, errors.NewInternalError("Unable to fetch models"))
		return
	}

	slog.Info("Retrieved models from server", "count", len(models))

	response := map[string]interface{}{
		"object": "list",
		"data":   models, // Empty list if no models found
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CompletionRequest represents a completion request
type CompletionRequest struct {
	Prompt      string  `json:"prompt"`
	Language    string  `json:"language,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	Stream      bool    `json:"stream,omitempty"`
}

// CompletionResponse represents a completion response
type CompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Text         string      `json:"text"`
		Index        int         `json:"index"`
		FinishReason string      `json:"finish_reason"`
		Logprobs     interface{} `json:"logprobs"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// handleCompletions handles completion requests
func (s *Server) handleCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteErrorResponse(w, errors.NewValidationError("Invalid JSON format"))
		return
	}

	if req.Prompt == "" {
		errors.WriteErrorResponse(w, errors.NewValidationError("Prompt is required"))
		return
	}

	ctx := r.Context()
	completion, err := s.copilotClient.GetCompletion(ctx, &copilot.CompletionRequest{
		Prompt:      req.Prompt,
		Language:    req.Language,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
	})
	if err != nil {
		if apiErr, ok := err.(*errors.APIError); ok {
			errors.WriteErrorResponse(w, apiErr)
		} else {
			errors.WriteErrorResponse(w, errors.NewInternalError(err.Error()))
		}
		return
	}

	// Create OpenAI-compatible response
	response := CompletionResponse{
		ID:      generateID(),
		Object:  "text_completion",
		Created: time.Now().Unix(),
		Model:   "copilot-codex",
		Choices: []struct {
			Text         string      `json:"text"`
			Index        int         `json:"index"`
			FinishReason string      `json:"finish_reason"`
			Logprobs     interface{} `json:"logprobs"`
		}{
			{
				Text:         completion,
				Index:        0,
				FinishReason: "stop",
				Logprobs:     nil,
			},
		},
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{
			PromptTokens:     estimateTokens(req.Prompt),
			CompletionTokens: estimateTokens(completion),
			TotalTokens:      estimateTokens(req.Prompt) + estimateTokens(completion),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ChatMessage represents a chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest represents a chat completion request
type ChatCompletionRequest struct {
	Model       string        `json:"model,omitempty"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
}

// ChatCompletionResponse represents a chat completion response
type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// handleChatCompletions handles chat completion requests
func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errors.WriteErrorResponse(w, errors.NewValidationError("Invalid JSON format"))
		return
	}

	if len(req.Messages) == 0 {
		errors.WriteErrorResponse(w, errors.NewValidationError("Messages are required"))
		return
	}

	// Convert chat messages to a simple prompt
	var prompt string
	for _, msg := range req.Messages {
		if msg.Role == "user" {
			prompt += msg.Content + "\n"
		}
	}

	ctx := r.Context()
	completion, err := s.copilotClient.GetCompletion(ctx, &copilot.CompletionRequest{
		Prompt:      prompt,
		Language:    "text",
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
	})
	if err != nil {
		if apiErr, ok := err.(*errors.APIError); ok {
			errors.WriteErrorResponse(w, apiErr)
		} else {
			errors.WriteErrorResponse(w, errors.NewInternalError(err.Error()))
		}
		return
	}

	// Create OpenAI-compatible response
	response := ChatCompletionResponse{
		ID:      generateID(),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   getDefaultOrString(req.Model, "gpt-4"),
		Choices: []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{
			{
				Index: 0,
				Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{
					Role:    "assistant",
					Content: completion,
				},
				FinishReason: "stop",
			},
		},
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{
			PromptTokens:     estimateTokens(prompt),
			CompletionTokens: estimateTokens(completion),
			TotalTokens:      estimateTokens(prompt) + estimateTokens(completion),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper functions
func generateID() string {
	return "reai-" + string(rune(time.Now().UnixNano()))
}

func estimateTokens(text string) int {
	// Simple token estimation (roughly 4 characters per token)
	return len(text) / 4
}

func getDefaultOrString(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
