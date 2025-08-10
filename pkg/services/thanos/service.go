package thanos

import (
	"context"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/services/thanos/integrations"
)

type ThanosStackDeploymentService struct {
	name            string
	deploymentRepo  DeploymentRepository
	stackRepo       StackRepository
	integrationRepo IntegrationRepository
	taskManager     TaskManager
	integrationMgr  *integrations.IntegrationManager
	logRepo         LogRepository
}

// taskManagerWrapper wraps TaskManager to match the interface expected by IntegrationManager
type taskManagerWrapper struct {
	taskManager TaskManager
}

func (tmw *taskManagerWrapper) AddTask(id string, task func(ctx context.Context)) {
	tmw.taskManager.AddTask(id, task)
}

func NewThanosService(
	deploymentRepo DeploymentRepository,
	stackRepo StackRepository,
	integrationRepo IntegrationRepository,
	taskManager TaskManager,
	logRepo LogRepository,
) *ThanosStackDeploymentService {
	taskManagerWrapper := &taskManagerWrapper{taskManager: taskManager}

	thanosDeploymentSrv := &ThanosStackDeploymentService{
		name:            "Thanos",
		deploymentRepo:  deploymentRepo,
		stackRepo:       stackRepo,
		integrationRepo: integrationRepo,
		taskManager:     taskManager,
		integrationMgr:  integrations.NewIntegrationManager(stackRepo, integrationRepo, taskManagerWrapper),
		logRepo:         logRepo,
	}

	thanosDeploymentSrv.taskManager.Start()

	return thanosDeploymentSrv
}

// InstallBridge installs a bridge for the given stack
func (s *ThanosStackDeploymentService) InstallBridge(ctx context.Context, stackId string) (*entities.Response, error) {
	return s.integrationMgr.InstallBridge(ctx, stackId)
}

// UninstallBridge uninstalls the bridge for the given stack
func (s *ThanosStackDeploymentService) UninstallBridge(ctx context.Context, stackId string) (*entities.Response, error) {
	return s.integrationMgr.UninstallBridge(ctx, stackId)
}

// InstallBlockExplorer installs a block explorer for the given stack
func (s *ThanosStackDeploymentService) InstallBlockExplorer(ctx context.Context, stackId string, request dtos.InstallBlockExplorerRequest) (*entities.Response, error) {
	return s.integrationMgr.InstallBlockExplorer(ctx, stackId, request)
}

// UninstallBlockExplorer uninstalls the block explorer for the given stack
func (s *ThanosStackDeploymentService) UninstallBlockExplorer(ctx context.Context, stackId string) (*entities.Response, error) {
	return s.integrationMgr.UninstallBlockExplorer(ctx, stackId)
}

// InstallMonitoring installs monitoring for the given stack
func (s *ThanosStackDeploymentService) InstallMonitoring(ctx context.Context, stackId uuid.UUID, request dtos.InstallMonitoringRequest) (*entities.Response, error) {
	return s.integrationMgr.InstallMonitoring(ctx, stackId, request)
}

// UninstallMonitoring uninstalls the monitoring for the given stack
func (s *ThanosStackDeploymentService) UninstallMonitoring(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	return s.integrationMgr.UninstallMonitoring(ctx, stackId)
}
