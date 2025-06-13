package routes

import (
	"github.com/gin-gonic/gin"
	handlers2 "github.com/tokamak-network/trh-backend/pkg/api/handlers"
	"github.com/tokamak-network/trh-backend/pkg/api/servers"
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
	handler := handlers2.NewHealthHandler()
	router.GET("", handler.GetHealth)
}

func setupThanosRoutes(router *gin.RouterGroup, server *servers.Server) {
	handler := handlers2.NewThanosHandler(server)
	router.POST("", handler.Deploy)
	router.POST("/:id/resume", handler.Resume)
	router.POST("/:id/stop", handler.Stop)
	router.POST("/:id/update", handler.UpdateNetwork)
	router.DELETE("/:id", handler.Terminate)
	router.GET("", handler.GetAllStacks)
	router.GET("/:id", handler.GetStackByID)
	router.POST("/:id/integrations/bridge", handler.InstallBridge)
	router.POST("/:id/integrations/block-explorer", handler.InstallBlockExplorer)
	router.DELETE("/:id/integrations/bridge", handler.UninstallBridge)
	router.DELETE("/:id/integrations/block-explorer", handler.UninstallBlockExplorer)
	router.GET("/:id/status", handler.GetStackStatus)
	router.GET("/:id/deployments", handler.GetDeployments)
	router.GET("/:id/integrations", handler.GetIntegrations)
	router.GET("/:id/integrations/:integrationId", handler.GetIntegrationById)
	router.GET("/:id/deployments/:deploymentId", handler.GetStackDeployment)
	router.GET("/:id/deployments/:deploymentId/status", handler.GetStackDeploymentStatus)
}
