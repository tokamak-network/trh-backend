package servers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	Router     *gin.Engine
	postgresDB *gorm.DB
}

func (s *Server) Start(port string) error {
	return s.Router.Run(":" + port)
}

func NewServer(db *gorm.DB) *Server {
	app := gin.Default()

	return &Server{
		Router:     app,
		postgresDB: db,
	}
}
