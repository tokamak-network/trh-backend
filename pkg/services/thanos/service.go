package thanos

import (
	"context"
	"errors"
	"fmt"
	"os"

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
		integrationMgr:  integrations.NewIntegrationManager(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManagerWrapper),
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

// RegisterCandidate delegates candidate registration to the integrations layer
func (s *ThanosStackDeploymentService) RegisterCandidate(ctx context.Context, stackId uuid.UUID, req dtos.RegisterCandidateRequest) (*entities.Response, error) {
	return s.integrationMgr.RegisterCandidateForStack(ctx, stackId, req)
}

// RegisterMetadataDAO delegates metadata dao registration to the integrations layer
func (s *ThanosStackDeploymentService) RegisterMetadataDAO(ctx context.Context, stackId uuid.UUID, req dtos.RegisterMetadataDAORequest) (*entities.Response, error) {
	return s.integrationMgr.RegisterMetadataDAOForStack(ctx, stackId, req)
}

// GetRegisterMetadataDAO delegates metadata dao registration to the integrations layer
func (s *ThanosStackDeploymentService) GetRegisterMetadataDAO(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	return s.integrationMgr.GetRegisterMetadataDAOForStack(ctx, stackId)
}

// DownloadDeploymentLogFile returns the deployment and validates that the log file exists for download
func (s *ThanosStackDeploymentService) DownloadDeploymentLogFile(stackId uuid.UUID, deploymentId uuid.UUID) (*entities.DeploymentEntity, error) {
	// Verify stack exists
	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return nil, err
	}
	if stack == nil {
		return nil, errors.New("stack not found")
	}

	// Get deployment
	deployment, err := s.deploymentRepo.GetDeploymentByID(deploymentId.String())
	if err != nil {
		return nil, err
	}
	if deployment == nil {
		return nil, errors.New("deployment not found")
	}

	// Verify deployment belongs to the specified stack
	if deployment.StackID == nil || *deployment.StackID != stackId {
		return nil, errors.New("deployment does not belong to the specified stack")
	}

	// Check if log file exists
	if deployment.LogPath == "" {
		return nil, errors.New("no log file available for this deployment")
	}

	// Verify file exists on filesystem
	if _, err := os.Stat(deployment.LogPath); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("log file not found on filesystem: ", deployment.LogPath)
			return nil, fmt.Errorf("log file not found on filesystem: %s", deployment.LogPath)
		}
		return nil, err
	}

	return deployment, nil
}

// GetRollupConfigFilePath returns the rollup config file path from stack metadata and validates it exists
func (s *ThanosStackDeploymentService) GetRollupConfigFilePath(stackId uuid.UUID) (string, error) {
	// Get stack
	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return "", err
	}
	if stack == nil {
		return "", errors.New("stack not found")
	}

	// Check if metadata exists
	if stack.Metadata == nil {
		return "", errors.New("stack metadata not found")
	}

	// Check if rollup config URL exists
	if stack.Metadata.RollupConfigUrl == "" {
		return "", errors.New("rollup config file not available for this stack")
	}

	// Verify file exists on filesystem
	if _, err := os.Stat(stack.Metadata.RollupConfigUrl); err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("rollup config file not found on filesystem")
		}
		return "", err
	}

	return stack.Metadata.RollupConfigUrl, nil
}
