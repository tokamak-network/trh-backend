package routes

import (
	"os"

	"github.com/gin-gonic/gin"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/tokamak-network/trh-backend/pkg/api/handlers"
	"github.com/tokamak-network/trh-backend/pkg/api/handlers/thanos"
	"github.com/tokamak-network/trh-backend/pkg/api/middleware"
	"github.com/tokamak-network/trh-backend/pkg/api/servers"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/repositories"
	"github.com/tokamak-network/trh-backend/pkg/services"

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

	// Initialize services with optimized configuration
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-in-production"
	}
	jwtService := services.NewJWTService(jwtSecret)
	authService := services.NewAuthService(userRepo, jwtService)

	// Create default admin account if no users exist
	if err := authService.CreateDefaultAdmin(); err != nil {
		logger.Fatal("Failed to create default admin account", zap.Error(err))
	}

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(authService)

	// Initialize middleware with optimized settings
	jwtMiddleware := middleware.NewJWTMiddleware(jwtService)

	// Health routes (public)
	setupHealthRoutes(router.Group("/health"))

	// Auth routes
	setupAuthRoutes(router.Group("/auth"), authHandler, jwtMiddleware)

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
	router.POST("/signup", authHandler.Signup)

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
		authenticatedRoutes.GET("/:id/deployments", handler.GetDeployments)
		authenticatedRoutes.GET("/:id/integrations", handler.GetIntegrations)
		authenticatedRoutes.GET("/:id/integrations/:integrationId", handler.GetIntegrationById)
		authenticatedRoutes.GET("/:id/deployments/:deploymentId", handler.GetStackDeployment)
		authenticatedRoutes.GET("/:id/deployments/:deploymentId/status", handler.GetStackDeploymentStatus)
	}
}
