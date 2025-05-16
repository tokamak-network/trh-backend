package server

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	router     *gin.Engine
	postgresDB *gorm.DB
}

func NewServer(db *gorm.DB) *Server {
	app := gin.Default()

	return &Server{
		router:     app,
		postgresDB: db,
	}
}

func (s *Server) Start(port string) error {
	return s.router.Run(":" + port)
}
