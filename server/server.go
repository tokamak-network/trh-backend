package server

import (
	"github.com/gin-gonic/gin"
)

type Server struct {
	router *gin.Engine
}

func NewServer() *Server {
	router := gin.Default()

	return &Server{
		router: router,
	}
}

func (s *Server) Start(port string) error {
	return s.router.Run(":" + port)
}
