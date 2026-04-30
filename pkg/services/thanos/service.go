package thanos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/services/thanos/integrations"
	stackThanos "github.com/tokamak-network/trh-backend/pkg/stacks/thanos"
	thanosSDKConstants "github.com/tokamak-network/trh-sdk/pkg/constants"
	"go.uber.org/zap"
)

type ThanosStackDeploymentService struct {
	name              string
	deploymentRepo    DeploymentRepository
	stackRepo         StackRepository
	integrationRepo   IntegrationRepository
	taskManager       TaskManager
	integrationMgr    *integrations.IntegrationManager
	logRepo           LogRepository
	ethClientFactory  EthClientFactory
}

// taskManagerWrapper wraps TaskManager to match the interface expected by IntegrationManager
type taskManagerWrapper struct {
	taskManager TaskManager
}

func (tmw *taskManagerWrapper) AddTask(id string, task func(ctx context.Context)) {
	tmw.taskManager.AddTask(id, task)
}

func (tmw *taskManagerWrapper) StopTask(id string) {
	tmw.taskManager.StopTask(id)
}

func (tmw *taskManagerWrapper) IsTaskRunning(id string) bool {
	return tmw.taskManager.IsTaskRunning(id)
}

func (tmw *taskManagerWrapper) AddTaskWithProgress(id string, task func(ctx context.Context, updateProgress func(string, float64))) {
	tmw.taskManager.AddTaskWithProgress(id, task)
}

func (tmw *taskManagerWrapper) SetTaskResult(id string, result any) {
	tmw.taskManager.SetTaskResult(id, result)
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
		name:             "Thanos",
		deploymentRepo:   deploymentRepo,
		stackRepo:        stackRepo,
		integrationRepo:  integrationRepo,
		taskManager:      taskManager,
		integrationMgr:   integrations.NewIntegrationManager(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManagerWrapper),
		logRepo:          logRepo,
		ethClientFactory: defaultEthClientFactory,
	}

	thanosDeploymentSrv.taskManager.Start()
	thanosDeploymentSrv.startBackupCleanupScheduler()
	go thanosDeploymentSrv.restoreAAOperators(context.Background())

	return thanosDeploymentSrv
}

func (s *ThanosStackDeploymentService) startBackupCleanupScheduler() {
	retentionDays := 14
	if v := os.Getenv("TRH_EFS_CLEANUP_RETENTION_DAYS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			retentionDays = parsed
		}
	}

	go func() {
		for {
			now := time.Now().UTC()
			next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, time.UTC)
			if !next.After(now) {
				next = next.Add(24 * time.Hour)
			}
			time.Sleep(time.Until(next))

			logger.Info("Starting daily backup cleanup", zap.Int("retention_days", retentionDays))
			stacks, err := s.stackRepo.GetAllStacks()
			if err != nil {
				logger.Error("Failed to list stacks for cleanup", zap.Error(err))
				continue
			}
			os.Setenv("TRH_EFS_CLEANUP_RETENTION_DAYS", strconv.Itoa(retentionDays))
			for _, stack := range stacks {
				if stack == nil || stack.Status != entities.StackStatusDeployed {
					continue
				}
				_, err := s.integrationMgr.BackupCleanup(context.Background(), stack.ID)
				if err != nil {
					logger.Error("Backup cleanup failed", zap.String("stackId", stack.ID.String()), zap.Error(err))
				}
			}
		}
	}()
}

func (s *ThanosStackDeploymentService) restoreAAOperators(ctx context.Context) {
	stacks, err := s.stackRepo.GetAllStacks()
	if err != nil {
		logger.Error("Failed to list stacks for AA operator restoration", zap.Error(err))
		return
	}

	for _, stack := range stacks {
		if stack == nil || stack.Status != entities.StackStatusDeployed {
			continue
		}
		if stack.DeploymentPath == "" {
			continue
		}

		sdkClient, err := stackThanos.NewThanosSDKClient(ctx, "", string(stack.Network), stack.DeploymentPath, false, "", "", "")
		if err != nil {
			logger.Warn("Failed to create SDK client for stack", zap.String("stackId", stack.ID.String()), zap.Error(err))
			continue
		}

		stackConfig := sdkClient.GetDeployConfig()
		if stackConfig == nil || !thanosSDKConstants.NeedsAASetup(stackConfig.Preset, stackConfig.FeeToken) {
			continue
		}

		stackID := stack.ID
		capturedClient := sdkClient
		s.taskManager.AddTask(
			fmt.Sprintf("aa-operator-%s", stackID.String()),
			func(ctx context.Context) {
				stackThanos.StartAAOperatorFromConfig(ctx, capturedClient)
			},
		)
	}
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

// GetContractsFilePath returns the contracts file path from stack metadata and validates it exists
func (s *ThanosStackDeploymentService) GetContractsFilePath(stackId uuid.UUID) (string, error) {
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

	contractsPath := stack.Metadata.ContractsPath

	// Check if contracts path exists
	if contractsPath == "" {
		contractsPath = filepath.Join(stack.DeploymentPath, "tokamak-thanos", "packages", "tokamak", "contracts-bedrock", "deployments", fmt.Sprintf("%d-deploy.json", stack.Metadata.L1ChainId))
	}

	// Verify file exists on filesystem
	if _, err := os.Stat(contractsPath); err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("contracts file not found on filesystem")
		}
		return "", err
	}

	return contractsPath, nil
}

// GetContractsFileContent returns the file bytes stored at the contracts path for the stack
func (s *ThanosStackDeploymentService) GetContractsFileContent(stackId uuid.UUID) (map[string]string, error) {
	filePath, err := s.GetContractsFilePath(stackId)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("contracts file not found on filesystem")
		}
		return nil, err
	}

	var contracts map[string]string
	if err := json.Unmarshal(content, &contracts); err != nil {
		return nil, errors.New("failed to convert the contracts")
	}

	contracts["L2CrossDomainMessengerProxy"] = "0x4200000000000000000000000000000000000007"
	return contracts, nil
}

// GetDeployConfigFilePath returns the deploy config file path from stack metadata and validates it exists
func (s *ThanosStackDeploymentService) GetDeployConfigFilePath(stackId uuid.UUID) (string, error) {
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

	deployConfigPath := stack.Metadata.DeployConfigPath

	// Check if deploy config path exists, set default if not
	if deployConfigPath == "" {
		deployConfigPath = filepath.Join(stack.DeploymentPath, "tokamak-thanos", "packages", "tokamak", "contracts-bedrock", "scripts", "deploy-config.json")
	}

	// Verify file exists on filesystem
	if _, err := os.Stat(deployConfigPath); err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("deploy config file not found on filesystem")
		}
		return "", err
	}

	return deployConfigPath, nil
}

// GetDeployConfigFileContent returns the file bytes stored at the deploy config path for the stack
func (s *ThanosStackDeploymentService) GetDeployConfigFileContent(stackId uuid.UUID) ([]byte, error) {
	filePath, err := s.GetDeployConfigFilePath(stackId)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("deploy config file not found on filesystem")
		}
		return nil, err
	}

	return content, nil
}

func (s *ThanosStackDeploymentService) BackupStatus(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	return s.integrationMgr.BackupStatus(ctx, stackId)
}

func (s *ThanosStackDeploymentService) BackupCheckpoints(ctx context.Context, stackId uuid.UUID, request dtos.BackupRequest) (*entities.Response, error) {
	return s.integrationMgr.BackupCheckpoints(ctx, stackId, request)
}

func (s *ThanosStackDeploymentService) BackupSnapshot(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	return s.integrationMgr.BackupSnapshot(ctx, stackId)
}

func (s *ThanosStackDeploymentService) BackupRestore(ctx context.Context, stackId uuid.UUID, request dtos.BackupRestoreRequest) (*entities.Response, error) {
	return s.integrationMgr.BackupRestore(ctx, stackId, request)
}

func (s *ThanosStackDeploymentService) BackupConfigure(ctx context.Context, stackId uuid.UUID, request dtos.BackupConfigureRequest) (*entities.Response, error) {
	return s.integrationMgr.BackupConfigure(ctx, stackId, request)
}

func (s *ThanosStackDeploymentService) BackupAttach(ctx context.Context, stackId uuid.UUID, request dtos.BackupAttachRequest) (*entities.Response, error) {
	return s.integrationMgr.BackupAttach(ctx, stackId, request)
}

func (s *ThanosStackDeploymentService) BackupPvPvcExport(ctx context.Context, stackId uuid.UUID) (string, string, error) {
	return s.integrationMgr.BackupPvPvcExport(ctx, stackId)
}

func (s *ThanosStackDeploymentService) BackupCleanup(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	return s.integrationMgr.BackupCleanup(ctx, stackId)
}

// DisableEmailAlert disables email alert configuration for the given stack
func (s *ThanosStackDeploymentService) DisableEmailAlert(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	return s.integrationMgr.DisableEmailAlert(ctx, stackId)
}

// DisableTelegramAlert disables telegram alert configuration for the given stack
func (s *ThanosStackDeploymentService) DisableTelegramAlert(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	return s.integrationMgr.DisableTelegramAlert(ctx, stackId)
}

// UpdateEmailAlert updates email alert configuration for the given stack
func (s *ThanosStackDeploymentService) UpdateEmailAlert(ctx context.Context, stackId uuid.UUID, request dtos.UpdateEmailConfigRequest) (*entities.Response, error) {
	return s.integrationMgr.UpdateEmailAlert(ctx, stackId, request)
}

// UpdateTelegramAlert updates telegram alert configuration for the given stack
func (s *ThanosStackDeploymentService) UpdateTelegramAlert(ctx context.Context, stackId uuid.UUID, request dtos.UpdateTelegramConfigRequest) (*entities.Response, error) {
	return s.integrationMgr.UpdateTelegramAlert(ctx, stackId, request)
}

func (s *ThanosStackDeploymentService) InstallCrossChainBridge(ctx context.Context, stackId uuid.UUID, request dtos.InstallCrossChainBridgeRequest) (*entities.Response, error) {
	return s.integrationMgr.InstallCrossChainBridge(ctx, stackId, request)
}

func (s *ThanosStackDeploymentService) UninstallCrossChainBridge(ctx context.Context, stackId uuid.UUID, mode string) (*entities.Response, error) {
	return s.integrationMgr.UninstallCrossChainBridge(ctx, stackId, mode)
}

func (s *ThanosStackDeploymentService) RegisterTokens(ctx context.Context, stackId uuid.UUID, mode string, request dtos.RegisterTokensAPIRequest) (*entities.Response, error) {
	return s.integrationMgr.RegisterTokens(ctx, stackId, mode, request)
}

func (s *ThanosStackDeploymentService) DeployNewL2Chain(ctx context.Context, stackId uuid.UUID, mode string, request dtos.DeployNewL2ChainRequest) (*entities.Response, error) {
	return s.integrationMgr.DeployNewL2Chain(ctx, stackId, mode, request)
}

// InstallUptimeService installs an uptime service for the given stack
func (s *ThanosStackDeploymentService) InstallUptimeService(ctx context.Context, stackId string) (*entities.Response, error) {
	return s.integrationMgr.InstallUptimeService(ctx, stackId)
}

// UninstallUptimeService uninstalls the uptime service for the given stack
func (s *ThanosStackDeploymentService) UninstallUptimeService(ctx context.Context, stackId string) (*entities.Response, error) {
	return s.integrationMgr.UninstallUptimeService(ctx, stackId)
}

// UninstallDRB uninstalls the DRB integration for the given stack
func (s *ThanosStackDeploymentService) UninstallDRB(ctx context.Context, stackId string) (*entities.Response, error) {
	return s.integrationMgr.UninstallDRB(ctx, stackId)
}

func (s *ThanosStackDeploymentService) CancelIntegration(ctx context.Context, stackId uuid.UUID, integrationId uuid.UUID) (*entities.Response, error) {
	return s.integrationMgr.CancelIntegration(ctx, stackId, integrationId)
}

func (s *ThanosStackDeploymentService) RetryIntegration(ctx context.Context, stackId uuid.UUID, integrationId uuid.UUID) (*entities.Response, error) {
	return s.integrationMgr.RetryIntegration(ctx, stackId, integrationId)
}
