package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tokamak-network/trh-backend/docs"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/routes"
	"github.com/tokamak-network/trh-backend/pkg/api/servers"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/connection"

	"github.com/gin-contrib/cors"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

//	@title			TRH Backend
//	@version		1.0
//	@description	TRH Backend API

//	@host		localhost:${PORT}
//	@BasePath	/api/v1

// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				Type "Bearer" followed by a space and JWT token.
func main() {
	logger.Init()

	// Load .env file if it exists (optional for Docker runtime)
	if err := godotenv.Load(".env"); err != nil {
		logger.Infof("No .env file found, using environment variables: %s", err)
	}

	port := getEnv("PORT", "8000")
	postgresUser := getEnv("POSTGRES_USER", "postgres")
	postgresHost := getEnv("POSTGRES_HOST", "localhost")
	postgresPassword := getEnv("POSTGRES_PASSWORD", "postgres")
	postgresDatabase := getEnv("POSTGRES_DB", "postgres")
	postgresPort := getEnv("POSTGRES_PORT", "5432")

	postgresDB, err := connection.Init(
		postgresUser,
		postgresHost,
		postgresPassword,
		postgresDatabase,
		postgresPort,
	)
	if err != nil {
		logger.Fatal("Failed to connect to postgres", zap.Error(err))
	}

	// programmatically set swagger info
	docs.SwaggerInfo.Title = "TRH Backend"
	docs.SwaggerInfo.Description = "TRH Backend API"
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.Schemes = []string{"http"}
	docs.SwaggerInfo.Host = fmt.Sprintf("localhost:%s", port)
	docs.SwaggerInfo.BasePath = "/api/v1"

	server := servers.NewServer(postgresDB)

	// Configure CORS with optimized settings
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}
	config.AllowHeaders = []string{"*"}
	config.MaxAge = 12 * time.Hour // Cache preflight requests

	server.Use(cors.New(config))

	routes.SetupRoutes(server)

	// Start server in a goroutine
	go func() {
		logger.Infof("Starting server on port %s", port)
		if err := server.Start(port); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Stop all in-progress deployments
	if err := server.Stop(); err != nil {
		logger.Error("Failed to stop deployments", zap.Error(err))
	}

	// Create a deadline for server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
