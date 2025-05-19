package routes

import (
	"trh-backend/internal/interfaces/api/handlers"
	"trh-backend/internal/interfaces/api/servers"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(server *servers.Server) {
	apiV1 := server.Router.Group("/api/v1")

	groupHealth := apiV1.Group("/health")
	GroupHealth(groupHealth, server)
}

func GroupHealth(group *gin.RouterGroup, server *servers.Server) {
	handler := handlers.NewHealthHandler(server)
	group.GET("", handler.GetHealth)
}
