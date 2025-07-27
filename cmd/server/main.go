package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devstroop/reai/internal/api"
	"github.com/devstroop/reai/internal/config"
	"github.com/devstroop/reai/internal/copilot"
)

func main() {
	// Initialize configuration
	cfg := config.LoadFromEnv()

	// Initialize structured logging
	logLevel := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		logLevel = slog.LevelDebug
	}
	
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	slog.Info("🚀 Starting ReAI - OpenAI Compatible API Server")
	slog.Info("📦 GitHub Copilot backend with OpenAI-style endpoints")
	slog.Info("🔧 Based on reverse-engineered Copilot API")
	slog.Info("📊 Configuration", "port", cfg.Port, "data_dir", cfg.DataDir)

	// Initialize Copilot client
	copilotClient, err := copilot.NewClient(cfg)
	if err != nil {
		slog.Error("Failed to create Copilot client", "error", err)
		os.Exit(1)
	}

	// Try to get session token (will trigger setup if needed)
	if err := copilotClient.GetSessionToken(context.Background()); err != nil {
		slog.Warn("Failed to get initial session token", "error", err)
		fmt.Println("⚠️  Authentication may be required on first API call")
	}

	// Start background token refresh
	go copilotClient.StartTokenRefresh(context.Background())

	// Create API server
	server := api.NewServer(copilotClient)
	
	// Setup HTTP server
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      server.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("✅ ReAI server initialized")
		slog.Info("🌐 Server running", "address", fmt.Sprintf("http://0.0.0.0:%d", cfg.Port))
		slog.Info("📊 Available endpoints:")
		slog.Info("   GET  /health              	- Health check")
		slog.Info("   GET  /v1/models           	- List available models")
		slog.Info("   POST /v1/completions      	- Code completions")
		slog.Info("   POST /v1/chat/completions 	- Chat/Q&A")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutdown signal received, stopping server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("Server stopped gracefully")
}
