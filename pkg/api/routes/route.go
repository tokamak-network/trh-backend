package routes

import (
	"github.com/gin-gonic/gin"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/tokamak-network/trh-backend/pkg/api/handlers"
	"github.com/tokamak-network/trh-backend/pkg/api/servers"

	swaggerFiles "github.com/swaggo/files"
)

func SetupRoutes(server *servers.Server) {
	apiV1 := server.Router.Group("/api/v1")
	setupV1Routes(apiV1, server)

	server.Router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

func setupV1Routes(router *gin.RouterGroup, server *servers.Server) {
	// Health routes
	setupHealthRoutes(router.Group("/health"))

	// Stack routes
	stacks := router.Group("/stacks")
	setupThanosRoutes(stacks.Group("/thanos"), server)
}

func setupHealthRoutes(router *gin.RouterGroup) {
	handler := handlers.NewHealthHandler()
	router.GET("", handler.GetHealth)
}

func setupThanosRoutes(router *gin.RouterGroup, server *servers.Server) {
	handler := handlers.NewThanosHandler(server)
	router.POST("", handler.Deploy)
	router.POST("/:id/resume", handler.Resume)
	router.POST("/:id/stop", handler.Stop)
	router.PUT("/:id", handler.UpdateNetwork)
	router.DELETE("/:id", handler.Terminate)
	router.GET("", handler.GetAllStacks)
	router.GET("/:id", handler.GetStackByID)
	router.POST("/:id/integrations/bridge", handler.InstallBridge)
	router.POST("/:id/integrations/block-explorer", handler.InstallBlockExplorer)
	router.POST("/:id/integrations/monitoring", handler.InstallMonitoring)
	router.POST("/:id/integrations/candidate-registry", handler.RegisterCandidates)
	router.DELETE("/:id/integrations/bridge", handler.UninstallBridge)
	router.DELETE("/:id/integrations/block-explorer", handler.UninstallBlockExplorer)
	router.DELETE("/:id/integrations/monitoring", handler.UninstallMonitoring)
	router.GET("/:id/status", handler.GetStackStatus)
	router.GET("/:id/deployments", handler.GetDeployments)
	router.GET("/:id/integrations", handler.GetIntegrations)
	router.GET("/:id/integrations/:integrationId", handler.GetIntegrationById)
	router.GET("/:id/deployments/:deploymentId", handler.GetStackDeployment)
	router.GET("/:id/deployments/:deploymentId/status", handler.GetStackDeploymentStatus)
}
