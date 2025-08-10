package thanos

import (
	"github.com/tokamak-network/trh-backend/pkg/api/servers"
	postgresRepositories "github.com/tokamak-network/trh-backend/pkg/infrastructure/postgres/repositories"
	"github.com/tokamak-network/trh-backend/pkg/services/thanos"
	"github.com/tokamak-network/trh-backend/pkg/taskmanager"
)

type ThanosDeploymentHandler struct {
	ThanosDeploymentService *thanos.ThanosStackDeploymentService
}

func NewThanosHandler(server *servers.Server) *ThanosDeploymentHandler {
	deploymentRepo := postgresRepositories.NewDeploymentRepository(server.PostgresDB)
	stackRepo := postgresRepositories.NewStackRepository(server.PostgresDB)
	integrationRepo := postgresRepositories.NewIntegrationRepository(server.PostgresDB)
	logRepo := postgresRepositories.NewLogRepository(server.PostgresDB)

	taskManager := taskmanager.NewTaskManager(5, 20)

	return &ThanosDeploymentHandler{
		ThanosDeploymentService: thanos.NewThanosService(deploymentRepo, stackRepo, integrationRepo, taskManager, logRepo),
	}
}
