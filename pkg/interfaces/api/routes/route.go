package routes

import (
	"trh-backend/pkg/interfaces/api/handlers"
	"trh-backend/pkg/interfaces/api/servers"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(server *servers.Server) {
	apiV1 := server.Router.Group("/api/v1")
	setupV1Routes(apiV1, server)

	healthGroup := server.Router.Group("/health")
	setupHealthRoutes(healthGroup)
}

func setupV1Routes(router *gin.RouterGroup, server *servers.Server) {
	stacks := router.Group("/stacks")
	setupThanosRoutes(stacks.Group("/thanos"), server)
}

func setupHealthRoutes(router *gin.RouterGroup) {
	handler := handlers.NewHealthHandler()
	router.GET("", handler.GetHealth)
}

func setupThanosRoutes(router *gin.RouterGroup, server *servers.Server) {
	handler := handlers.NewThanosHandler(server)
	router.POST("", handler.DeployThanos)
	router.POST("/:id/resume", handler.ResumeThanos)
	router.DELETE("/:id", handler.TerminateThanos)
}
