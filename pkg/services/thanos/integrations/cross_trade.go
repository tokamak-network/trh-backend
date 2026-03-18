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

// CrossTradeBridgeIntegration handles bridge installation and uninstallation
type CrossTradeBridgeIntegration struct {
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
		GetIntegrationById(id string) (*entities.IntegrationEntity, error)
	}
	logRepo interface {
		CreateLog(log *entities.LogEntity) error
	}
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
		StopTask(id string)
		IsTaskRunning(id string) bool
	}
}

// NewCrossTradeBridgeIntegration creates a new bridge integration handler
func NewCrossTradeBridgeIntegration(
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
		GetIntegrationById(id string) (*entities.IntegrationEntity, error)
	},
	logRepo interface {
		CreateLog(log *entities.LogEntity) error
	},
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
		StopTask(id string)
		IsTaskRunning(id string) bool
	},
) *CrossTradeBridgeIntegration {
	return &CrossTradeBridgeIntegration{
		stackRepo:       stackRepo,
		deploymentRepo:  deploymentRepo,
		integrationRepo: integrationRepo,
		logRepo:         logRepo,
		taskManager:     taskManager,
	}
}

// Install installs a cross trade for the given stack
func (b *CrossTradeBridgeIntegration) Install(ctx context.Context, stackUUID uuid.UUID, request dtos.InstallCrossChainBridgeRequest) (*entities.Response, error) {
	stackId := stackUUID.String()
	if err := request.Validate(); err != nil {
		logger.Error("invalid cross trade bridge request", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Invalid cross trade bridge request",
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

	// check if cross trade is already in non-terminated state
	integrations, err := b.integrationRepo.GetActiveIntegrations(stackId, "cross-trade")
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", "cross-trade"), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if len(integrations) > 0 {
		logger.Error("There is already an active cross trade", zap.String("plugin", "cross-trade"))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "There is already an active cross trade",
			Data:    nil,
		}, nil
	}

	logPath := utils.GetLogPath(stack.ID, "cross-trade")

	crossTradeBridgeIntegration := &entities.IntegrationEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Type:    enum.IntegrationTypeCrossTrade.String(),
		Status:  string(entities.DeploymentStatusPending),
		Config:  []byte("{}"),
		LogPath: logPath,
	}

	if err := b.integrationRepo.CreateIntegration(crossTradeBridgeIntegration); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("install-cross-trade-%s", stackId)
	b.taskManager.AddTask(taskId, func(ctx context.Context) {
		b.installTask(ctx, stack, request, logPath)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

// Uninstall uninstalls the cross trade for the given stack
func (b *CrossTradeBridgeIntegration) Uninstall(ctx context.Context, stackId string) (*entities.Response, error) {
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

	logPath := utils.GetLogPath(stack.ID, "uninstall-cross-trade")

	CrossTradeBridgeIntegration, _ := b.integrationRepo.GetInstalledIntegration(stack.ID.String(), enum.IntegrationTypeCrossTrade.String())
	if CrossTradeBridgeIntegration == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "cross trade integration not found",
			Data:    nil,
		}, nil
	}

	if err := b.integrationRepo.UpdateIntegrationStatus(CrossTradeBridgeIntegration.ID.String(), entities.DeploymentStatusPending); err != nil {
		logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("uninstall-cross-trade-%s", stackId)
	b.taskManager.AddTask(taskId, func(ctx context.Context) {
		b.uninstallTask(ctx, CrossTradeBridgeIntegration.ID, stack, stackId, logPath)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

// installTask handles the actual installation process
func (b *CrossTradeBridgeIntegration) installTask(ctx context.Context, stack *entities.StackEntity, request dtos.InstallCrossChainBridgeRequest, logPath string) {
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	CrossTradeBridgeIntegration, err := b.integrationRepo.GetInstalledIntegration(stack.ID.String(), enum.IntegrationTypeCrossTrade.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()), zap.Error(err))
		return
	}

	if err := b.integrationRepo.UpdateIntegrationStatus(CrossTradeBridgeIntegration.ID.String(), entities.DeploymentStatusInProgress); err != nil {
		logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()), zap.Error(err))
		return
	}

	configBytes, err := json.Marshal(request)
	if err != nil {
		logger.Error("failed to marshal cross trade config", zap.Error(err))
		return
	}

	// Create deployment record for installing cross trade
	deployment := &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    constants.InstallCrossTradeBridgeStep,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  configBytes,
	}
	if err := b.deploymentRepo.CreateDeployment(deployment); err != nil {
		logger.Error("failed to create deployment record", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()), zap.Error(err))
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
	crossTradeIntegrationOutput, err := thanos.InstallCrossTradeBridge(ctx, sdkClient, &request)
	if err != nil {
		logger.Error("failed to install cross trade", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()), zap.Error(err))
		if updateErr := b.integrationRepo.UpdateIntegrationStatusWithReason(CrossTradeBridgeIntegration.ID.String(), entities.DeploymentStatusFailed, err.Error()); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()), zap.Error(updateErr), zap.String("integrationId", CrossTradeBridgeIntegration.ID.String()))
		}
		deploymentStatus := entities.DeploymentRunStatusFailed
		if errors.Is(err, context.Canceled) {
			deploymentStatus = entities.DeploymentRunStatusStopped
		}
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), deploymentStatus)
		return
	}
	crossTradeIntegrationOutputURL := crossTradeIntegrationOutput.DeployCrossTradeApplicationOutput.URL

	if crossTradeIntegrationOutputURL == "" {
		logger.Error("cross trade URL is empty", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()))
		if updateErr := b.integrationRepo.UpdateIntegrationStatusWithReason(CrossTradeBridgeIntegration.ID.String(), entities.DeploymentStatusFailed, "cross trade URL is empty"); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()), zap.Error(updateErr), zap.String("integrationId", CrossTradeBridgeIntegration.ID.String()))
		}
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	logger.Debug("cross trade successfully installed", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()), zap.String("url", crossTradeIntegrationOutputURL))

	if err = b.integrationRepo.UpdateConfig(CrossTradeBridgeIntegration.ID.String(), json.RawMessage(configBytes)); err != nil {
		logger.Error("failed to update cross trade integration config", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()), zap.Error(err))
		return
	}

	crossTradeIntegrationMetadata := map[string]interface{}{
		"url":       crossTradeIntegrationOutputURL,
		"contracts": crossTradeIntegrationOutput.DeployCrossTradeContractsOutput,
	}
	metadataBytes, err := json.Marshal(crossTradeIntegrationMetadata)
	if err != nil {
		logger.Error("failed to marshal cross trade metadata", zap.Error(err))
		return
	}

	if err = b.integrationRepo.UpdateMetadataAfterInstalled(CrossTradeBridgeIntegration.ID.String(), entities.IntegrationInfo(metadataBytes)); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()), zap.Error(err))
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	if stack.Metadata == nil {
		stack.Metadata = &entities.StackMetadata{}
	}

	stack.Metadata.CrossTradeUrl = crossTradeIntegrationOutputURL
	if err = b.stackRepo.UpdateMetadata(stack.ID.String(), stack.Metadata); err != nil {
		logger.Error("failed to update stack metadata", zap.String("stackId", stack.ID.String()), zap.Error(err))
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusSuccess)
}

// uninstallTask handles the actual uninstallation process
func (b *CrossTradeBridgeIntegration) uninstallTask(ctx context.Context, integrationID uuid.UUID, stack *entities.StackEntity, stackId string, logPath string) {
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	var uninstallDeployment *entities.DeploymentEntity
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic during cross-trade uninstall", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()), zap.Any("recover", r))
			if uninstallDeployment != nil {
				_ = b.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
			}
			_ = b.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, fmt.Sprint(r))
		}
	}()

	if err := b.integrationRepo.UpdateIntegrationStatus(integrationID.String(), entities.DeploymentStatusTerminating); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()), zap.Error(err))
		return
	}

	// Create deployment record for uninstalling cross trade
	uninstallDeployment = &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    constants.UninstallCrossTradeBridgeStep,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  []byte("{}"),
	}
	if err := b.deploymentRepo.CreateDeployment(uninstallDeployment); err != nil {
		logger.Error("failed to create uninstall deployment record", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()), zap.Error(err))
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

	if err = thanos.UninstallCrossTradeBridge(ctx, sdkClient); err != nil {
		logger.Error("failed to uninstall cross-trade", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()), zap.Error(err))
		_ = b.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
		_ = b.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, err.Error())
		return
	}

	if err = b.integrationRepo.UpdateIntegrationStatus(integrationID.String(), entities.DeploymentStatusTerminated); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", enum.IntegrationTypeCrossTrade.String()), zap.Error(err))
		return
	}

	stack.Metadata.CrossTradeUrl = ""
	if err = b.stackRepo.UpdateMetadata(stackId, stack.Metadata); err != nil {
		logger.Error("failed to update stack metadata", zap.String("stackId", stackId), zap.Error(err))
		return
	}

	_ = b.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusSuccess)
}

// Cancel cancels an in-progress cross-trade installation and cleans up AWS resources
func (b *CrossTradeBridgeIntegration) Cancel(ctx context.Context, stackId uuid.UUID, integrationId uuid.UUID) (*entities.Response, error) {
	integration, err := b.integrationRepo.GetIntegrationById(integrationId.String())
	if err != nil {
		logger.Error("failed to get integration", zap.Error(err), zap.String("integrationId", integrationId.String()))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if integration == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Integration not found",
			Data:    nil,
		}, nil
	}

	if integration.Status != string(entities.DeploymentStatusInProgress) && integration.Status != string(entities.DeploymentStatusPending) {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Can only cancel installations that are in progress or pending",
			Data:    nil,
		}, nil
	}

	if err = b.integrationRepo.UpdateIntegrationStatusWithReason(
		integration.ID.String(),
		entities.DeploymentStatusCancelling,
		"Stopping installation process. This may take a few minutes to safely clean up AWS resources.",
	); err != nil {
		return &entities.Response{Status: http.StatusInternalServerError, Message: "Failed to update status"}, err
	}

	b.taskManager.AddTask(fmt.Sprintf("cancel-cross-trade-%s", stackId.String()), func(ctx context.Context) {
		taskId := fmt.Sprintf("install-cross-trade-%s", stackId.String())
		b.taskManager.StopTask(taskId)

		stack, err := b.stackRepo.GetStackByID(stackId.String())
		if err != nil {
			logger.Error("failed to get stack", zap.Error(err), zap.String("stackId", stackId.String()))
			return
		}
		stackConfig := dtos.DeployThanosRequest{}
		if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
			logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
			return
		}

		sdkClient, err := thanos.NewThanosSDKClient(
			ctx,
			utils.GetLogPath(stack.ID, "cancel-cross-trade"),
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

		if err = thanos.UninstallCrossTradeBridge(ctx, sdkClient); err != nil {
			logger.Error("failed to uninstall cross-trade during cancel", zap.Error(err))
			_ = b.integrationRepo.UpdateIntegrationStatusWithReason(
				integration.ID.String(),
				entities.DeploymentStatusFailed,
				err.Error(),
			)
			return
		}

		logger.Info("Cancellation completed successfully", zap.String("integrationId", integrationId.String()))
		_ = b.integrationRepo.UpdateIntegrationStatusWithReason(
			integration.ID.String(),
			entities.DeploymentStatusCancelled,
			"Installation cancelled successfully. All AWS resources have been cleaned up.",
		)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Cancellation in progress. Installation will be stopped and AWS resources will be cleaned up. This may take a few minutes.",
		Data:    nil,
	}, nil
}

// Retry retries a cancelled cross-trade installation
func (b *CrossTradeBridgeIntegration) Retry(ctx context.Context, stackId uuid.UUID, integrationId uuid.UUID) (*entities.Response, error) {
	return retryIntegrationCommon(ctx, stackId, integrationId, b.integrationRepo, b.stackRepo,
		func(stack *entities.StackEntity, integration *entities.IntegrationEntity) error {
			logPath := utils.GetLogPath(stack.ID, fmt.Sprintf("install-%s", integration.Type))

			if integration.Config == nil || len(integration.Config) == 0 || string(integration.Config) == "{}" {
				logger.Error("installation config is missing or empty", zap.String("integrationId", integrationId.String()))
				return &BadRequestError{message: "Cannot retry installation: original configuration not found. Please uninstall and reinstall instead."}
			}

			var request dtos.InstallCrossChainBridgeRequest
			if err := json.Unmarshal(integration.Config, &request); err != nil {
				logger.Error("failed to unmarshal config", zap.Error(err), zap.String("integrationId", integrationId.String()))
				return fmt.Errorf("failed to retrieve installation config")
			}

			taskId := fmt.Sprintf("install-cross-trade-%s", stackId.String())
			b.taskManager.AddTask(taskId, func(ctx context.Context) {
				b.installTask(ctx, stack, request, logPath)
			})

			return nil
		})
}

// tailAndIngestLogs tails a log file and ingests each line into the database
func (b *CrossTradeBridgeIntegration) tailAndIngestLogs(
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
