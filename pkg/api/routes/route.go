package routes

import (
	"os"

	"github.com/gin-gonic/gin"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/tokamak-network/trh-backend/pkg/api/handlers"
	configurationHandlers "github.com/tokamak-network/trh-backend/pkg/api/handlers/configuration"
	"github.com/tokamak-network/trh-backend/pkg/api/handlers/thanos"
	"github.com/tokamak-network/trh-backend/pkg/api/middleware"
	"github.com/tokamak-network/trh-backend/pkg/api/servers"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/repositories"
	"github.com/tokamak-network/trh-backend/pkg/services"
	"github.com/tokamak-network/trh-backend/pkg/services/configuration"

	swaggerFiles "github.com/swaggo/files"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"go.uber.org/zap"
)

func SetupRoutes(server *servers.Server) {
	apiV1 := server.Router.Group("/api/v1")
	setupV1Routes(apiV1, server)

	server.Router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

func setupV1Routes(router *gin.RouterGroup, server *servers.Server) {
	// Initialize repositories with connection pooling
	userRepo := repositories.NewUserRepository(server.PostgresDB)
	awsCredentialsRepo := repositories.NewAWSCredentialsRepository(server.PostgresDB)
	rpcUrlRepo := repositories.NewRPCUrlRepository(server.PostgresDB)
	apiKeyRepo := repositories.NewApiKeyRepository(server.PostgresDB)

	// Initialize services with optimized configuration
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-in-production"
	}
	jwtService := services.NewJWTService(jwtSecret)
	authService := services.NewAuthService(userRepo, jwtService)
	awsCredentialsService := configuration.NewAWSCredentialsService(awsCredentialsRepo)
	rpcUrlService := configuration.NewRPCUrlService(rpcUrlRepo)
	apiKeyService := configuration.NewApiKeyService(apiKeyRepo)

	// Create default admin account if no users exist
	if err := authService.CreateDefaultAdmin(); err != nil {
		logger.Fatal("Failed to create default admin account", zap.Error(err))
	}

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService)
	awsCredentialsHandler := configurationHandlers.NewAWSCredentialsHandler(awsCredentialsService)
	rpcUrlHandler := configurationHandlers.NewRPCUrlHandler(rpcUrlService)
	apiKeyHandler := configurationHandlers.NewApiKeyHandler(apiKeyService)

	// Initialize middleware with optimized settings
	jwtMiddleware := middleware.NewJWTMiddleware(jwtService)

	// Health routes (public)
	setupHealthRoutes(router.Group("/health"))

	// Auth routes
	setupAuthRoutes(router.Group("/auth"), authHandler, jwtMiddleware)

	// Configuration routes (protected)
	setupConfigurationRoutes(router.Group("/configuration"), awsCredentialsHandler, rpcUrlHandler, apiKeyHandler, jwtMiddleware)

	// Stack routes (protected)
	stacks := router.Group("/stacks")
	setupThanosRoutes(stacks.Group("/thanos"), server, jwtMiddleware)
}

func setupHealthRoutes(router *gin.RouterGroup) {
	handler := handlers.NewHealthHandler()
	router.GET("", handler.GetHealth)
}

func setupAuthRoutes(router *gin.RouterGroup, authHandler *handlers.AuthHandler, jwtMiddleware *middleware.JWTMiddleware) {
	// Public routes
	router.POST("/login", authHandler.Login)

	// Protected routes (any authenticated user)
	protected := router.Group("")
	protected.Use(jwtMiddleware.AuthMiddleware())
	{
		protected.GET("/profile", authHandler.GetProfile)
	}

	// Admin routes (admin role required)
	admin := router.Group("")
	admin.Use(jwtMiddleware.AuthMiddleware(entities.UserRoleAdmin))
	{
		admin.GET("/users", authHandler.GetUsers)
	}
}

func setupConfigurationRoutes(router *gin.RouterGroup, awsCredentialsHandler *configurationHandlers.AWSCredentialsHandler, rpcUrlHandler *configurationHandlers.RPCUrlHandler, apiKeyHandler *configurationHandlers.ApiKeyHandler, jwtMiddleware *middleware.JWTMiddleware) {
	// AWS Credentials sub-routes
	awsCredentialsRoutes := router.Group("/aws-credentials")
	setupAWSCredentialsSubRoutes(awsCredentialsRoutes, awsCredentialsHandler, jwtMiddleware)

	// RPC URL sub-routes
	rpcUrlRoutes := router.Group("/rpc-url")
	setupRPCUrlSubRoutes(rpcUrlRoutes, rpcUrlHandler, jwtMiddleware)

	// API Key sub-routes
	apiKeyRoutes := router.Group("/api-key")
	setupAPIKeySubRoutes(apiKeyRoutes, apiKeyHandler, jwtMiddleware)
}

func setupAWSCredentialsSubRoutes(router *gin.RouterGroup, awsCredentialsHandler *configurationHandlers.AWSCredentialsHandler, jwtMiddleware *middleware.JWTMiddleware) {
	// Admin-only routes (require admin role)
	adminRoutes := router.Group("")
	adminRoutes.Use(jwtMiddleware.AuthMiddleware(entities.UserRoleAdmin))
	{
		adminRoutes.POST("", awsCredentialsHandler.Create)
		adminRoutes.PATCH("/:id", awsCredentialsHandler.Update)
		adminRoutes.DELETE("/:id", awsCredentialsHandler.Delete)
		adminRoutes.POST("/regions", awsCredentialsHandler.GetAvailableRegions)
	}

	// Authenticated routes (require valid JWT token - any role)
	authenticatedRoutes := router.Group("")
	authenticatedRoutes.Use(jwtMiddleware.AuthMiddleware())
	{
		authenticatedRoutes.GET("", awsCredentialsHandler.GetAll)
		authenticatedRoutes.GET("/:id", awsCredentialsHandler.GetByID)
	}
}

func setupRPCUrlSubRoutes(router *gin.RouterGroup, rpcUrlHandler *configurationHandlers.RPCUrlHandler, jwtMiddleware *middleware.JWTMiddleware) {
	// Admin-only routes (require admin role)
	adminRoutes := router.Group("")
	adminRoutes.Use(jwtMiddleware.AuthMiddleware(entities.UserRoleAdmin))
	{
		adminRoutes.POST("", rpcUrlHandler.Create)
		adminRoutes.PATCH("/:id", rpcUrlHandler.Update)
		adminRoutes.DELETE("/:id", rpcUrlHandler.Delete)
	}

	// Authenticated routes (require valid JWT token - any role)
	authenticatedRoutes := router.Group("")
	authenticatedRoutes.Use(jwtMiddleware.AuthMiddleware())
	{
		authenticatedRoutes.GET("", rpcUrlHandler.GetAll)
		authenticatedRoutes.GET("/:id", rpcUrlHandler.GetByID)
	}
}

func setupAPIKeySubRoutes(router *gin.RouterGroup, apiKeyHandler *configurationHandlers.ApiKeyHandler, jwtMiddleware *middleware.JWTMiddleware) {
	// Admin-only routes (require admin role)
	adminRoutes := router.Group("")
	adminRoutes.Use(jwtMiddleware.AuthMiddleware(entities.UserRoleAdmin))
	{
		adminRoutes.POST("", apiKeyHandler.Create)
		adminRoutes.PATCH("/:id", apiKeyHandler.Update)
		adminRoutes.DELETE("/:id", apiKeyHandler.Delete)
	}

	// Authenticated routes (require valid JWT token - any role)
	authenticatedRoutes := router.Group("")
	authenticatedRoutes.Use(jwtMiddleware.AuthMiddleware())
	{
		authenticatedRoutes.GET("", apiKeyHandler.GetAll)
		authenticatedRoutes.GET("/:id", apiKeyHandler.GetByID)
	}
}

func setupThanosRoutes(router *gin.RouterGroup, server *servers.Server, jwtMiddleware *middleware.JWTMiddleware) {
	handler := thanos.NewThanosHandler(server)

	// Admin-only routes (require admin role)
	adminRoutes := router.Group("")
	adminRoutes.Use(jwtMiddleware.AuthMiddleware(entities.UserRoleAdmin))
	{
		// Stack management operations
		adminRoutes.POST("", handler.Deploy)
		adminRoutes.DELETE("/:id", handler.Terminate)
		adminRoutes.PUT("/:id", handler.UpdateNetwork)

		// Stack control operations
		adminRoutes.POST("/:id/resume", handler.Resume)
		adminRoutes.POST("/:id/stop", handler.Stop)

		// Integration management
		adminRoutes.POST("/:id/integrations/bridge", handler.InstallBridge)
		adminRoutes.POST("/:id/integrations/block-explorer", handler.InstallBlockExplorer)
		adminRoutes.POST("/:id/integrations/monitoring", handler.InstallMonitoring)
		adminRoutes.POST("/:id/integrations/register-candidate", handler.RegisterCandidates)
		adminRoutes.POST("/:id/integrations/register-metadata-dao", handler.RegisterMetadataDAO)
		adminRoutes.GET("/:id/integrations/register-metadata-dao", handler.GetRegisterMetadataDAO)
		adminRoutes.DELETE("/:id/integrations/bridge", handler.UninstallBridge)
		adminRoutes.DELETE("/:id/integrations/block-explorer", handler.UninstallBlockExplorer)
		adminRoutes.DELETE("/:id/integrations/monitoring", handler.UninstallMonitoring)
	}

	// Authenticated routes (require valid JWT token - any role)
	authenticatedRoutes := router.Group("")
	authenticatedRoutes.Use(jwtMiddleware.AuthMiddleware())
	{
		// Read-only operations
		authenticatedRoutes.GET("", handler.GetAllStacks)
		authenticatedRoutes.GET("/:id", handler.GetStackByID)
		authenticatedRoutes.GET("/:id/status", handler.GetStackStatus)
		authenticatedRoutes.GET("/:id/rollupconfig", handler.DownloadRollupConfig)
		authenticatedRoutes.GET("/:id/deployments", handler.GetDeployments)
		authenticatedRoutes.GET("/:id/integrations", handler.GetIntegrations)
		authenticatedRoutes.GET("/:id/integrations/:integrationId", handler.GetIntegrationById)
		authenticatedRoutes.GET("/:id/deployments/:deploymentId", handler.GetStackDeployment)
		authenticatedRoutes.GET("/:id/deployments/:deploymentId/status", handler.GetStackDeploymentStatus)
		authenticatedRoutes.GET("/:id/deployments/:deploymentId/logs", handler.GetDeploymentLogs)
		authenticatedRoutes.GET("/:id/deployments/:deploymentId/logs/download", handler.DownloadDeploymentLogFile)
		authenticatedRoutes.GET("/:id/logs", handler.GetStackLogs)
	}
}
