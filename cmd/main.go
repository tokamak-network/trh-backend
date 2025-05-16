package main

import (
	"log"
	"os"
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

	server := server.NewServer()

	server.Start(port)

}
