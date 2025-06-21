package main

import (
	"fmt"
	"log"
	"os"

	"github.com/tokamak-network/trh-backend/docs"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/routes"
	"github.com/tokamak-network/trh-backend/pkg/api/servers"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/connection"

	"github.com/gin-contrib/cors"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	_ "github.com/tokamak-network/trh-backend/docs"
)

// @title           TRH Backend
// @version         1.0
// @description     TRH Backend API

// @host      localhost:${PORT}
// @BasePath  /api/v1

// @securityDefinitions.basic  NoAuth
func main() {

	logger.Init()

	// Load .env file if it exists (optional for Docker runtime)
	if err := godotenv.Load(".env"); err != nil {
		logger.Infof("No .env file found, using environment variables: %s", err)
	}

	port := os.Getenv("PORT")

	if port == "" {
		port = "8000"
	}

	postgresUser := os.Getenv("POSTGRES_USER")
	postgresHost := os.Getenv("POSTGRES_HOST")
	postgresPassword := os.Getenv("POSTGRES_PASSWORD")
	postgresDatabase := os.Getenv("POSTGRES_DB")
	postgresPort := os.Getenv("POSTGRES_PORT")

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
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"*"}

	server.Use(cors.New(config))

	routes.SetupRoutes(server)

	err = server.Start(port)
	if err != nil {
		logger.Error("Failed to start server", zap.Error(err))
		log.Fatal(err)
	}
}
