package main

import (
	"log"
	"os"

	"trh-backend/internal/logger"
	"trh-backend/pkg/infrastructure/postgres/connection"
	"trh-backend/pkg/interfaces/api/routes"
	"trh-backend/pkg/interfaces/api/servers"

	"github.com/gin-contrib/cors"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {

	logger.Init()

	err := godotenv.Load(".env")
	if err != nil {
		logger.Error("Failed to load environment.", zap.Error(err))
		return
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

	postgresDB := connection.Init(
		postgresUser,
		postgresHost,
		postgresPassword,
		postgresDatabase,
		postgresPort,
	)

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
