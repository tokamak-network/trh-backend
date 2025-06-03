package servers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	Router     *gin.Engine
	PostgresDB *gorm.DB
}

func (s *Server) Start(port string) error {
	return s.Router.Run(":" + port)
}

func (s *Server) Use(middleware gin.HandlerFunc) {
	s.Router.Use(middleware)
}

func NewServer(db *gorm.DB) *Server {
	app := gin.Default()

	return &Server{
		Router:     app,
		PostgresDB: db,
	}
}
