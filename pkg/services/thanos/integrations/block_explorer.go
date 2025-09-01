package integrations

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/constants"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/enum"
	"github.com/tokamak-network/trh-backend/pkg/stacks/thanos"
	"go.uber.org/zap"
)

// BlockExplorerIntegration handles block explorer installation and uninstallation
type BlockExplorerIntegration struct {
	stackRepo interface {
		GetStackByID(id string) (*entities.StackEntity, error)
		UpdateMetadata(id string, metadata *entities.StackMetadata) error
	}
	deploymentRepo interface {
		CreateDeployment(deployment *entities.DeploymentEntity) error
		UpdateDeploymentStatus(deploymentId string, status entities.DeploymentRunStatus) error
	}
	integrationRepo interface {
		GetActiveIntegrations(stackId, integrationType string) ([]*entities.IntegrationEntity, error)
		CreateIntegration(integration *entities.IntegrationEntity) error
		UpdateIntegrationStatus(id string, status entities.DeploymentStatus) error
		UpdateIntegrationStatusWithReason(id string, status entities.DeploymentStatus, reason string) error
		GetInstalledIntegration(stackId, integrationType string) (*entities.IntegrationEntity, error)
		UpdateConfig(id string, config json.RawMessage) error
		UpdateMetadataAfterInstalled(id string, metadata entities.IntegrationInfo) error
	}
	logRepo interface {
		CreateLog(log *entities.LogEntity) error
	}
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
	}
}

// NewBlockExplorerIntegration creates a new block explorer integration handler
func NewBlockExplorerIntegration(
	stackRepo interface {
		GetStackByID(id string) (*entities.StackEntity, error)
		UpdateMetadata(id string, metadata *entities.StackMetadata) error
	},
	deploymentRepo interface {
		CreateDeployment(deployment *entities.DeploymentEntity) error
		UpdateDeploymentStatus(deploymentId string, status entities.DeploymentRunStatus) error
	},
	integrationRepo interface {
		GetActiveIntegrations(stackId, integrationType string) ([]*entities.IntegrationEntity, error)
		CreateIntegration(integration *entities.IntegrationEntity) error
		UpdateIntegrationStatus(id string, status entities.DeploymentStatus) error
		UpdateIntegrationStatusWithReason(id string, status entities.DeploymentStatus, reason string) error
		GetInstalledIntegration(stackId, integrationType string) (*entities.IntegrationEntity, error)
		UpdateConfig(id string, config json.RawMessage) error
		UpdateMetadataAfterInstalled(id string, metadata entities.IntegrationInfo) error
	},
	logRepo interface {
		CreateLog(log *entities.LogEntity) error
	},
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
	},
) *BlockExplorerIntegration {
	return &BlockExplorerIntegration{
		stackRepo:       stackRepo,
		deploymentRepo:  deploymentRepo,
		integrationRepo: integrationRepo,
		logRepo:         logRepo,
		taskManager:     taskManager,
	}
}

// Install installs a block explorer for the given stack
func (b *BlockExplorerIntegration) Install(ctx context.Context, stackId string, request dtos.InstallBlockExplorerRequest) (*entities.Response, error) {
	if err := request.Validate(); err != nil {
		logger.Error("invalid block explorer request", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Invalid block explorer request",
			Data:    nil,
		}, err
	}

	stack, err := b.stackRepo.GetStackByID(stackId)
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deployed, yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	if stack == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Stack not found",
			Data:    nil,
		}, nil
	}

	// check if block explorer is already in non-terminated state
	integrations, err := b.integrationRepo.GetActiveIntegrations(stackId, "block-explorer")
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", "block-explorer"), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if len(integrations) > 0 {
		logger.Error("There is already an active block explorer", zap.String("plugin", "block-explorer"))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "There is already an active block explorer",
			Data:    nil,
		}, nil
	}

	logPath := utils.GetLogPath(stack.ID, "block-explorer")

	blockExplorerIntegration := &entities.IntegrationEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Type:    enum.IntegrationTypeBlockExplorer.String(),
		Status:  string(entities.DeploymentStatusPending),
		Config:  []byte("{}"),
		LogPath: logPath,
	}

	if err := b.integrationRepo.CreateIntegration(blockExplorerIntegration); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("install-block-explorer-%s", stackId)
	b.taskManager.AddTask(taskId, func(ctx context.Context) {
		b.installTask(ctx, stack, request, logPath)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

// Uninstall uninstalls the block explorer for the given stack
func (b *BlockExplorerIntegration) Uninstall(ctx context.Context, stackId string) (*entities.Response, error) {
	stack, err := b.stackRepo.GetStackByID(stackId)
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if stack == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Stack not found",
			Data:    nil,
		}, nil
	}

	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stackId), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	logPath := utils.GetLogPath(stack.ID, "uninstall-block-explorer")

	blockExplorerIntegration, _ := b.integrationRepo.GetInstalledIntegration(stack.ID.String(), enum.IntegrationTypeBlockExplorer.String())
	if blockExplorerIntegration == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Block explorer integration not found",
			Data:    nil,
		}, nil
	}

	if err := b.integrationRepo.UpdateIntegrationStatus(blockExplorerIntegration.ID.String(), entities.DeploymentStatusPending); err != nil {
		logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("uninstall-block-explorer-%s", stackId)
	b.taskManager.AddTask(taskId, func(ctx context.Context) {
		b.uninstallTask(ctx, stack, stackId, logPath)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

// installTask handles the actual installation process
func (b *BlockExplorerIntegration) installTask(ctx context.Context, stack *entities.StackEntity, request dtos.InstallBlockExplorerRequest, logPath string) {
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	blockExplorerIntegration, err := b.integrationRepo.GetInstalledIntegration(stack.ID.String(), enum.IntegrationTypeBlockExplorer.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
		return
	}

	if err := b.integrationRepo.UpdateIntegrationStatus(blockExplorerIntegration.ID.String(), entities.DeploymentStatusInProgress); err != nil {
		logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
		return
	}

	configBytes, err := json.Marshal(request)
	if err != nil {
		logger.Error("failed to marshal block explorer config", zap.Error(err))
		return
	}

	// Create deployment record for installing block explorer
	deployment := &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    constants.InstallBlockExplorerStep,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  configBytes,
	}
	if err := b.deploymentRepo.CreateDeployment(deployment); err != nil {
		logger.Error("failed to create deployment record", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
		return
	}

	// Start log ingestion for this plugin installation
	ingestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go b.tailAndIngestLogs(ingestCtx, stack.ID, deployment.ID, logPath)

	sdkClient, err := thanos.NewThanosSDKClient(
		ctx,
		logPath,
		string(stack.Network),
		stack.DeploymentPath,
		stackConfig.RegisterCandidate,
		stackConfig.AwsAccessKey,
		stackConfig.AwsSecretAccessKey,
		stackConfig.AwsRegion,
	)
	if err != nil {
		logger.Error("failed to create thanos sdk client", zap.Error(err))
		return
	}
	blockExplorerUrl, err := thanos.InstallBlockExplorer(ctx, sdkClient, &request)
	if err != nil {
		logger.Error("failed to install block explorer", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
		if updateErr := b.integrationRepo.UpdateIntegrationStatusWithReason(blockExplorerIntegration.ID.String(), entities.DeploymentStatusFailed, err.Error()); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(updateErr), zap.String("integrationId", blockExplorerIntegration.ID.String()))
		}
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	if blockExplorerUrl == "" {
		logger.Error("block explorer URL is empty", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()))
		if updateErr := b.integrationRepo.UpdateIntegrationStatusWithReason(blockExplorerIntegration.ID.String(), entities.DeploymentStatusFailed, "Block explorer URL is empty"); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(updateErr), zap.String("integrationId", blockExplorerIntegration.ID.String()))
		}
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	logger.Debug("block explorer successfully installed", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.String("url", blockExplorerUrl))

	if err = b.integrationRepo.UpdateConfig(blockExplorerIntegration.ID.String(), json.RawMessage(configBytes)); err != nil {
		logger.Error("failed to update block explorer integration config", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
		return
	}

	blockExplorerMetadata := map[string]string{"url": blockExplorerUrl}
	metadataBytes, err := json.Marshal(blockExplorerMetadata)
	if err != nil {
		logger.Error("failed to marshal block explorer metadata", zap.Error(err))
		return
	}

	if err = b.integrationRepo.UpdateMetadataAfterInstalled(blockExplorerIntegration.ID.String(), entities.IntegrationInfo(metadataBytes)); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	stack.Metadata.ExplorerUrl = blockExplorerUrl
	if err = b.stackRepo.UpdateMetadata(stack.ID.String(), stack.Metadata); err != nil {
		logger.Error("failed to update stack metadata", zap.String("stackId", stack.ID.String()), zap.Error(err))
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusSuccess)
}

// uninstallTask handles the actual uninstallation process
func (b *BlockExplorerIntegration) uninstallTask(ctx context.Context, stack *entities.StackEntity, stackId string, logPath string) {
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	var uninstallDeployment *entities.DeploymentEntity
	var integration *entities.IntegrationEntity
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic during block-explorer uninstall", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Any("recover", r))
			if uninstallDeployment != nil {
				_ = b.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
			}
			if integration != nil {
				_ = b.integrationRepo.UpdateIntegrationStatusWithReason(integration.ID.String(), entities.DeploymentStatusFailed, fmt.Sprint(r))
			}
		}
	}()

	integration, err := b.integrationRepo.GetInstalledIntegration(stackId, enum.IntegrationTypeBlockExplorer.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
		return
	}

	if integration == nil {
		logger.Error("integration not found", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()))
		return
	}

	if err = b.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminating); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
		return
	}

	// Create deployment record for uninstalling block explorer
	uninstallDeployment = &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    constants.UninstallBlockExplorerStep,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  []byte("{}"),
	}
	if err := b.deploymentRepo.CreateDeployment(uninstallDeployment); err != nil {
		logger.Error("failed to create uninstall deployment record", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
		return
	}

	// Start log ingestion for this plugin uninstallation
	ingestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go b.tailAndIngestLogs(ingestCtx, stack.ID, uninstallDeployment.ID, logPath)

	sdkClient, err := thanos.NewThanosSDKClient(
		ctx,
		logPath,
		string(stack.Network),
		stack.DeploymentPath,
		stackConfig.RegisterCandidate,
		stackConfig.AwsAccessKey,
		stackConfig.AwsSecretAccessKey,
		stackConfig.AwsRegion,
	)
	if err != nil {
		logger.Error("failed to create thanos sdk client", zap.Error(err))
		return
	}

	if err = thanos.UninstallBlockExplorer(ctx, sdkClient); err != nil {
		logger.Error("failed to uninstall block-explorer", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
		_ = b.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
		_ = b.integrationRepo.UpdateIntegrationStatusWithReason(integration.ID.String(), entities.DeploymentStatusFailed, err.Error())
		return
	}

	if err = b.integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusTerminated); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeBlockExplorer.String()), zap.Error(err))
		return
	}

	stack.Metadata.ExplorerUrl = ""
	if err = b.stackRepo.UpdateMetadata(stackId, stack.Metadata); err != nil {
		logger.Error("failed to update stack metadata", zap.String("stackId", stackId), zap.Error(err))
		return
	}

	_ = b.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusSuccess)
}

// tailAndIngestLogs tails a log file and ingests each line into the database
func (b *BlockExplorerIntegration) tailAndIngestLogs(
	ctx context.Context,
	stackID uuid.UUID,
	deploymentID uuid.UUID,
	logPath string,
) {
	// Wait for file to appear
	for {
		if _, err := os.Stat(logPath); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}

	f, err := os.Open(logPath)
	if err != nil {
		logger.Error("failed to open log file", zap.String("path", logPath), zap.Error(err))
		return
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := reader.ReadString('\n')
			if len(line) > 0 {
				msg := strings.TrimRight(line, "\r\n")
				if msg != "" {
					l := &entities.LogEntity{
						StackID:      &stackID,
						DeploymentID: &deploymentID,
						Message:      msg,
					}
					if dbErr := b.logRepo.CreateLog(l); dbErr != nil {
						logger.Error("failed to insert log", zap.Error(dbErr))
					}
				}
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					time.Sleep(300 * time.Millisecond)
					continue
				}
				logger.Error("error reading log file", zap.Error(err))
				return
			}
		}
	}
}
