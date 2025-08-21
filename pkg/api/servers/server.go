package servers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tokamak-network/trh-backend/pkg/api/middleware"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	postgresRepositories "github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/repositories"
	"github.com/tokamak-network/trh-backend/pkg/services/thanos"
	"github.com/tokamak-network/trh-backend/pkg/taskmanager"
	"gorm.io/gorm"
)

type Server struct {
	Router     *gin.Engine
	PostgresDB *gorm.DB
	server     *http.Server
}

func (s *Server) Start(port string) error {
	// Configure Gin for production
	gin.SetMode(gin.ReleaseMode)

	// Create HTTP server with optimized settings
	s.server = &http.Server{
		Addr:         ":" + port,
		Handler:      s.Router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *Server) Use(middleware gin.HandlerFunc) {
	s.Router.Use(middleware)
}

func NewServer(db *gorm.DB) *Server {
	// Configure Gin for better performance
	gin.SetMode(gin.ReleaseMode)

	app := gin.New()

	// Use optimized middleware
	app.Use(gin.Recovery())
	app.Use(middleware.RequestLoggerMiddleware())

	return &Server{
		Router:     app,
		PostgresDB: db,
	}
}

func (s *Server) Stop() error {
	deploymentRepo := postgresRepositories.NewDeploymentRepository(s.PostgresDB)
	stackRepo := postgresRepositories.NewStackRepository(s.PostgresDB)
	integrationRepo := postgresRepositories.NewIntegrationRepository(s.PostgresDB)
	logRepo := postgresRepositories.NewLogRepository(s.PostgresDB)

	taskManager := taskmanager.NewTaskManager(5, 20)

	thanosService := thanos.NewThanosService(deploymentRepo, stackRepo, integrationRepo, taskManager, logRepo)

	stacks, err := stackRepo.GetAllStacks()
	if err != nil {
		return err
	}

	for _, stack := range stacks {
		if stack.Status == entities.StackStatusDeploying {
			_, err = thanosService.StopDeployingThanosStack(context.Background(), stack.ID)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
