package routes

import (
	"github.com/tokamak-network/trh-backend/pkg/interfaces/api/handlers"
	"github.com/tokamak-network/trh-backend/pkg/interfaces/api/servers"

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
	router.GET("", handler.GetAllStacks)
	router.GET("/:id", handler.GetStackByID)
	router.GET("/:id/status", handler.GetStackStatus)
	router.GET("/:id/deployments", handler.GetStackDeployments)
	router.GET("/:id/deployments/:deploymentId", handler.GetStackDeployment)
	router.GET("/:id/deployments/:deploymentId/status", handler.GetStackDeploymentStatus)
}
