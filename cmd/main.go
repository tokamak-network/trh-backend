package main

import (
	"log"
	"os"
	"trh-backend/db"
	"trh-backend/server"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Println("Failed to load environment.")
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

	postgresDB := db.Init(postgresUser, postgresHost, postgresPassword, postgresDatabase, postgresPort)

	server := server.NewServer(postgresDB)

	server.Start(port)
}
