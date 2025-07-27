package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// APIError represents different types of API errors
type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	return e.Message
}

// Error types
var (
	ErrAuthentication = &APIError{Type: "authentication_error", Message: "Authentication failed", Code: http.StatusUnauthorized}
	ErrTokenExpired   = &APIError{Type: "token_expired", Message: "Token expired or invalid", Code: http.StatusUnauthorized}
	ErrRateLimit      = &APIError{Type: "rate_limit", Message: "Rate limit exceeded", Code: http.StatusTooManyRequests}
	ErrValidation     = &APIError{Type: "validation_error", Message: "Request validation failed", Code: http.StatusBadRequest}
	ErrCopilotAPI     = &APIError{Type: "copilot_api_error", Message: "GitHub Copilot API error", Code: http.StatusBadGateway}
	ErrNetwork        = &APIError{Type: "network_error", Message: "Network communication failed", Code: http.StatusBadGateway}
	ErrJSONParsing    = &APIError{Type: "json_error", Message: "Invalid JSON format", Code: http.StatusBadRequest}
	ErrIO             = &APIError{Type: "io_error", Message: "File operation failed", Code: http.StatusInternalServerError}
	ErrJWT            = &APIError{Type: "jwt_error", Message: "Token validation failed", Code: http.StatusUnauthorized}
	ErrInternal       = &APIError{Type: "internal_error", Message: "Internal server error", Code: http.StatusInternalServerError}
)

// NewAuthenticationError creates a new authentication error with custom message
func NewAuthenticationError(message string) *APIError {
	return &APIError{
		Type:    "authentication_error",
		Message: fmt.Sprintf("Authentication failed: %s", message),
		Code:    http.StatusUnauthorized,
	}
}

// NewValidationError creates a new validation error with custom message
func NewValidationError(message string) *APIError {
	return &APIError{
		Type:    "validation_error",
		Message: fmt.Sprintf("Request validation failed: %s", message),
		Code:    http.StatusBadRequest,
	}
}

// NewCopilotAPIError creates a new Copilot API error with custom message
func NewCopilotAPIError(message string) *APIError {
	return &APIError{
		Type:    "copilot_api_error",
		Message: fmt.Sprintf("GitHub Copilot API error: %s", message),
		Code:    http.StatusBadGateway,
	}
}

// NewInternalError creates a new internal error with custom message
func NewInternalError(message string) *APIError {
	return &APIError{
		Type:    "internal_error",
		Message: fmt.Sprintf("Internal server error: %s", message),
		Code:    http.StatusInternalServerError,
	}
}

// WriteErrorResponse writes an error response to the HTTP response writer
func WriteErrorResponse(w http.ResponseWriter, err *APIError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.Code)
	
	response := map[string]interface{}{
		"error": err,
	}
	
	json.NewEncoder(w).Encode(response)
}

// WrapError converts a generic error to an APIError
func WrapError(err error) *APIError {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr
	}
	return NewInternalError(err.Error())
}
