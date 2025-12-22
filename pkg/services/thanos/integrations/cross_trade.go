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
	thanosConstants "github.com/tokamak-network/trh-sdk/pkg/constants"
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
		GetIntegrationByStatus(
			stackId string,
			integrationType string,
			status entities.DeploymentStatus,
		) (*entities.IntegrationEntity, error)
		GetIntegrationById(id string) (*entities.IntegrationEntity, error)
	}
	logRepo interface {
		CreateLog(log *entities.LogEntity) error
	}
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
		StopTask(id string)
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
		GetIntegrationByStatus(
			stackId string,
			integrationType string,
			status entities.DeploymentStatus,
		) (*entities.IntegrationEntity, error)
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

	var integrationType string
	if request.Mode == "l2_to_l1" {
		integrationType = enum.IntegrationTypeCrossTradeL2ToL1.String()
	} else if request.Mode == "l2_to_l2" {
		integrationType = enum.IntegrationTypeCrossTradeL2ToL2.String()
	} else {
		logger.Error("invalid cross trade mode", zap.String("mode", string(request.Mode)))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Invalid cross trade mode",
			Data:    nil,
		}, nil
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
	integrations, err := b.integrationRepo.GetActiveIntegrations(stackId, integrationType)
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", integrationType), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if len(integrations) > 0 {
		logger.Error("There is already an active integration", zap.String("plugin", integrationType))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: fmt.Sprintf("There is already an active %s", integrationType),
			Data:    nil,
		}, nil
	}

	logPath := utils.GetLogPath(stack.ID, fmt.Sprintf("install-%s", integrationType))

	crossTradeBridgeIntegration := &entities.IntegrationEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Type:    integrationType,
		Status:  string(entities.DeploymentStatusPending),
		Config:  []byte("{}"),
		LogPath: logPath,
	}

	if err := b.integrationRepo.CreateIntegration(crossTradeBridgeIntegration); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", integrationType), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("install-%s-%s", integrationType, stackId)
	b.taskManager.AddTask(taskId, func(ctx context.Context) {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
		b.installTask(ctxWithTimeout, stack, request, logPath, integrationType)
	})
	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

// Uninstall uninstalls the cross trade for the given stack
func (b *CrossTradeBridgeIntegration) Uninstall(ctx context.Context, stackId string, mode string) (*entities.Response, error) {
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

	var integrationType string
	if mode == "l2_to_l1" {
		integrationType = enum.IntegrationTypeCrossTradeL2ToL1.String()
	} else if mode == "l2_to_l2" {
		integrationType = enum.IntegrationTypeCrossTradeL2ToL2.String()
	} else {
		logger.Error("invalid cross trade mode", zap.String("mode", mode))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Invalid cross trade mode",
			Data:    nil,
		}, nil
	}

	installedIntegration, _ := b.integrationRepo.GetInstalledIntegration(stack.ID.String(), integrationType)
	if installedIntegration == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Cross trade integration not found",
			Data:    nil,
		}, nil
	}

	if err := b.integrationRepo.UpdateIntegrationStatus(installedIntegration.ID.String(), entities.DeploymentStatusPending); err != nil {
		logger.Error("failed to update integration status", zap.String("plugin", installedIntegration.Type), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("uninstall-cross-trade-%s", stackId)
	b.taskManager.AddTask(taskId, func(ctx context.Context) {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
		b.uninstallTask(ctxWithTimeout, installedIntegration.ID, stack, stackId, logPath, installedIntegration.Type)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

// installTask handles the actual installation process
func (b *CrossTradeBridgeIntegration) installTask(ctx context.Context, stack *entities.StackEntity, request dtos.InstallCrossChainBridgeRequest, logPath string, integrationType string) {
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	pendingIntegration, err := b.integrationRepo.GetIntegrationByStatus(stack.ID.String(), integrationType, entities.DeploymentStatusPending)
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", integrationType), zap.Error(err))
		return
	}

	if pendingIntegration == nil {
		logger.Error("pending integration not found", zap.String("plugin", integrationType))
		return
	}

	if err := b.integrationRepo.UpdateIntegrationStatus(pendingIntegration.ID.String(), entities.DeploymentStatusInProgress); err != nil {
		logger.Error("failed to update integration status", zap.String("plugin", integrationType), zap.Error(err))
		return
	}

	configBytes, err := json.Marshal(request)
	if err != nil {
		logger.Error("failed to marshal cross trade config", zap.Error(err))
		return
	}

	var step string
	if integrationType == enum.IntegrationTypeCrossTradeL2ToL1.String() {
		step = constants.InstallCrossTradeL2L1Step
	} else if integrationType == enum.IntegrationTypeCrossTradeL2ToL2.String() {
		step = constants.InstallCrossTradeL2L2Step
	} else {
		logger.Error("invalid cross trade mode", zap.String("mode", integrationType))
		return
	}

	// Create deployment record for installing cross trade
	deployment := &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    step,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  configBytes,
	}
	if err := b.deploymentRepo.CreateDeployment(deployment); err != nil {
		logger.Error("failed to create deployment record", zap.String("plugin", integrationType), zap.Error(err))
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

		// Update integration status to failed
		if updateErr := b.integrationRepo.UpdateIntegrationStatusWithReason(pendingIntegration.ID.String(), entities.DeploymentStatusFailed, err.Error()); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", integrationType), zap.Error(updateErr), zap.String("integrationId", pendingIntegration.ID.String()))
		}

		// Update deployment status to failed
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}
	output, err := thanos.InstallCrossTradeBridge(ctx, sdkClient, &request)
	if err != nil {
		logger.Error("failed to install cross trade", zap.String("plugin", integrationType), zap.Error(err))
		if updateErr := b.integrationRepo.UpdateIntegrationStatusWithReason(pendingIntegration.ID.String(), entities.DeploymentStatusFailed, err.Error()); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", integrationType), zap.Error(updateErr), zap.String("integrationId", pendingIntegration.ID.String()))
		}
		deploymentStatus := entities.DeploymentRunStatusFailed
		if utils.IsContextCanceled(err) {
			deploymentStatus = entities.DeploymentRunStatusStopped
		}
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), deploymentStatus)
		return
	}
	logger.Info("cross trade successfully installed", zap.Any("output", output))
	crossTradeIntegrationOutputURL := output.DeployCrossTradeApplicationOutput.URL

	if crossTradeIntegrationOutputURL == "" {
		logger.Error("cross trade URL is empty", zap.String("plugin", integrationType))
		if updateErr := b.integrationRepo.UpdateIntegrationStatusWithReason(pendingIntegration.ID.String(), entities.DeploymentStatusFailed, "cross trade URL is empty"); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", integrationType), zap.Error(updateErr), zap.String("integrationId", pendingIntegration.ID.String()))
		}
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	logger.Debug("cross trade successfully installed", zap.String("plugin", integrationType), zap.String("url", crossTradeIntegrationOutputURL))

	if err = b.integrationRepo.UpdateConfig(pendingIntegration.ID.String(), json.RawMessage(configBytes)); err != nil {
		logger.Error("failed to update cross trade integration config", zap.String("plugin", integrationType), zap.Error(err))
		return
	}

	crossTradeIntegrationMetadata := map[string]interface{}{
		"url":       crossTradeIntegrationOutputURL,
		"contracts": output.DeployCrossTradeContractsOutput,
	}
	metadataBytes, err := json.Marshal(crossTradeIntegrationMetadata)
	if err != nil {
		logger.Error("failed to marshal cross trade metadata", zap.Error(err))
		return
	}

	if err = b.integrationRepo.UpdateMetadataAfterInstalled(pendingIntegration.ID.String(), entities.IntegrationInfo(metadataBytes)); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", integrationType), zap.Error(err))
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	if stack.Metadata == nil {
		stack.Metadata = &entities.StackMetadata{}
	}

	if integrationType == enum.IntegrationTypeCrossTradeL2ToL1.String() {
		stack.Metadata.L2L1CrossTradeUrl = crossTradeIntegrationOutputURL
	} else if integrationType == enum.IntegrationTypeCrossTradeL2ToL2.String() {
		stack.Metadata.L2L2CrossTradeUrl = crossTradeIntegrationOutputURL
	} else {
		logger.Error("invalid cross trade mode", zap.String("mode", integrationType))
		return
	}

	if err = b.stackRepo.UpdateMetadata(stack.ID.String(), stack.Metadata); err != nil {
		logger.Error("failed to update stack metadata", zap.String("stackId", stack.ID.String()), zap.Error(err))
		_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	_ = b.deploymentRepo.UpdateDeploymentStatus(deployment.ID.String(), entities.DeploymentRunStatusSuccess)
}

// uninstallTask handles the actual uninstallation process
func (b *CrossTradeBridgeIntegration) uninstallTask(ctx context.Context, integrationID uuid.UUID, stack *entities.StackEntity, stackId string, logPath string, integrationType string) {
	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stack.ID.String()), zap.Error(err))
		return
	}

	var uninstallDeployment *entities.DeploymentEntity
	defer func() {
		if r := recover(); r != nil {
			logger.Error("panic during cross-trade uninstall", zap.String("plugin", integrationType), zap.Any("recover", r))
			if uninstallDeployment != nil {
				_ = b.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
			}
			_ = b.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, fmt.Sprint(r))
		}
	}()

	if err := b.integrationRepo.UpdateIntegrationStatus(integrationID.String(), entities.DeploymentStatusTerminating); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", integrationType), zap.Error(err))
		return
	}

	var step string
	if integrationType == enum.IntegrationTypeCrossTradeL2ToL1.String() {
		step = constants.UninstallCrossTradeL2L1Step
	} else if integrationType == enum.IntegrationTypeCrossTradeL2ToL2.String() {
		step = constants.UninstallCrossTradeL2L2Step
	} else {
		logger.Error("invalid cross trade mode", zap.String("mode", integrationType))
		return
	}

	// Create deployment record for uninstalling cross trade
	uninstallDeployment = &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    step,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  []byte("{}"),
	}
	if err := b.deploymentRepo.CreateDeployment(uninstallDeployment); err != nil {
		logger.Error("failed to create uninstall deployment record", zap.String("plugin", integrationType), zap.Error(err))
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
		if updateErr := b.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, err.Error()); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", integrationType), zap.Error(updateErr), zap.String("integrationId", integrationID.String()))
		}
		_ = b.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	var mode string
	if integrationType == enum.IntegrationTypeCrossTradeL2ToL1.String() {
		mode = string(thanosConstants.CrossTradeDeployModeL2ToL1)
	} else if integrationType == enum.IntegrationTypeCrossTradeL2ToL2.String() {
		mode = string(thanosConstants.CrossTradeDeployModeL2ToL2)
	} else {
		logger.Error("invalid cross trade mode", zap.String("mode", integrationType))
		return
	}
	if err = thanos.UninstallCrossTradeBridge(ctx, sdkClient, mode); err != nil {
		logger.Error("failed to uninstall cross-trade", zap.String("plugin", integrationType), zap.Error(err))
		if err := b.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusFailed); err != nil {
			logger.Error("failed to update deployment status", zap.String("plugin", integrationType), zap.Error(err), zap.String("deploymentId", uninstallDeployment.ID.String()))
		}
		if updateErr := b.integrationRepo.UpdateIntegrationStatusWithReason(integrationID.String(), entities.DeploymentStatusFailed, err.Error()); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", integrationType), zap.Error(updateErr), zap.String("integrationId", integrationID.String()))
		}
		return
	}

	if err = b.integrationRepo.UpdateIntegrationStatus(integrationID.String(), entities.DeploymentStatusTerminated); err != nil {
		logger.Error("failed to update integration", zap.String("plugin", integrationType), zap.Error(err))
		return
	}

	if integrationType == enum.IntegrationTypeCrossTradeL2ToL1.String() {
		stack.Metadata.L2L1CrossTradeUrl = ""
	} else if integrationType == enum.IntegrationTypeCrossTradeL2ToL2.String() {
		stack.Metadata.L2L2CrossTradeUrl = ""
	} else {
		logger.Error("invalid cross trade mode", zap.String("mode", integrationType))
		return
	}
	if err = b.stackRepo.UpdateMetadata(stackId, stack.Metadata); err != nil {
		logger.Error("failed to update stack metadata", zap.String("stackId", stackId), zap.Error(err))
		return
	}

	_ = b.deploymentRepo.UpdateDeploymentStatus(uninstallDeployment.ID.String(), entities.DeploymentRunStatusSuccess)
}

func (b *CrossTradeBridgeIntegration) RegisterTokens(
	ctx context.Context,
	stackId uuid.UUID,
	mode string,
	request dtos.RegisterTokensAPIRequest,
) (*entities.Response, error) {
	stack, err := b.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error("failed to get stack", zap.String("stackId", stackId.String()), zap.Error(err))
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
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	var pluginType string
	if mode == string(thanosConstants.CrossTradeDeployModeL2ToL1) {
		pluginType = string(enum.IntegrationTypeCrossTradeL2ToL1)
	} else if mode == string(thanosConstants.CrossTradeDeployModeL2ToL2) {
		pluginType = string(enum.IntegrationTypeCrossTradeL2ToL2)
	} else {
		logger.Error("invalid cross trade mode", zap.String("mode", mode))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Invalid cross trade mode",
			Data:    nil,
		}, errors.New("invalid cross trade mode")
	}

	crossTradeIntegration, err := b.integrationRepo.GetInstalledIntegration(stackId.String(), pluginType)
	if err != nil {
		logger.Error("failed to get cross trade integration", zap.String("stackId", stackId.String()), zap.String("pluginType", pluginType), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Failed to get cross trade integration",
			Data:    nil,
		}, err
	}
	if crossTradeIntegration == nil {
		logger.Error("cross trade integration not found", zap.String("stackId", stackId.String()), zap.String("pluginType", pluginType))
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Cross trade integration not found",
			Data:    nil,
		}, nil
	}
	logPath := utils.GetLogPath(stack.ID, "register-tokens")

	crossTradeMode := thanosConstants.CrossTradeDeployMode(mode)
	// Create deployment record for uninstalling cross trade
	var registerTokensDeployment *entities.DeploymentEntity
	var integration *entities.IntegrationEntity
	defer func() {
		if r := recover(); r != nil {
			if registerTokensDeployment != nil {
				_ = b.deploymentRepo.UpdateDeploymentStatus(registerTokensDeployment.ID.String(), entities.DeploymentRunStatusFailed)
			}
			if integration != nil {
				_ = b.integrationRepo.UpdateIntegrationStatusWithReason(integration.ID.String(), entities.DeploymentStatusFailed, fmt.Sprint(r))
			}
		}
	}()
	var step string
	if mode == string(thanosConstants.CrossTradeDeployModeL2ToL1) {
		step = constants.RegisterTokensL2L1Step
	} else if mode == string(thanosConstants.CrossTradeDeployModeL2ToL2) {
		step = constants.RegisterTokensL2L2Step
	} else {
		logger.Error("invalid cross trade mode", zap.String("mode", mode))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Invalid cross trade mode",
			Data:    nil,
		}, errors.New("invalid cross trade mode")
	}
	registerTokensDeployment = &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    step,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  []byte("{}"),
	}
	if err := b.deploymentRepo.CreateDeployment(registerTokensDeployment); err != nil {
		logger.Error("failed to create register tokens deployment record", zap.String("mode", mode), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Failed to create deployment record",
			Data:    nil,
		}, err
	}

	b.taskManager.AddTask(fmt.Sprintf("register-tokens-%s-%s", stackId.String(), mode), func(ctx context.Context) {
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
			logger.Error("failed to create thanos sdk client", zap.String("stackId", stackId.String()), zap.Error(err))
			_ = b.deploymentRepo.UpdateDeploymentStatus(registerTokensDeployment.ID.String(), entities.DeploymentRunStatusFailed)
			return
		}
		ctxWithTimeout, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
		output, err := thanos.RegisterTokens(ctxWithTimeout, sdkClient, crossTradeMode, request.Tokens)
		if err != nil {
			logger.Error("failed to register tokens", zap.String("stackId", stackId.String()), zap.String("mode", mode), zap.Error(err))
			_ = b.deploymentRepo.UpdateDeploymentStatus(registerTokensDeployment.ID.String(), entities.DeploymentRunStatusFailed)
			return
		}

		// Update integration metadata
		metadata := map[string]interface{}{
			"contracts":         output.DeployCrossTradeContractsOutput,
			"url":               output.DeployCrossTradeApplicationOutput.URL,
			"registered_tokens": output.RegisterTokens,
		}
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			logger.Error("failed to marshal cross trade integration metadata", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
		if err := b.integrationRepo.UpdateMetadataAfterInstalled(crossTradeIntegration.ID.String(), entities.IntegrationInfo(metadataBytes)); err != nil {
			logger.Error("failed to update cross trade integration metadata", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}

		_ = b.deploymentRepo.UpdateDeploymentStatus(registerTokensDeployment.ID.String(), entities.DeploymentRunStatusSuccess)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (b *CrossTradeBridgeIntegration) DeployNewL2Chain(
	ctx context.Context,
	stackId uuid.UUID,
	mode string,
	request dtos.DeployNewL2ChainRequest,
) (*entities.Response, error) {
	stack, err := b.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error("failed to get stack", zap.String("stackId", stackId.String()), zap.Error(err))
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

	var pluginType string
	if mode == string(thanosConstants.CrossTradeDeployModeL2ToL1) {
		pluginType = string(enum.IntegrationTypeCrossTradeL2ToL1)
	} else if mode == string(thanosConstants.CrossTradeDeployModeL2ToL2) {
		pluginType = string(enum.IntegrationTypeCrossTradeL2ToL2)
	} else {
		logger.Error("invalid cross trade mode", zap.String("mode", mode))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Invalid cross trade mode",
			Data:    nil,
		}, errors.New("invalid cross trade mode")
	}

	crossTradeIntegration, err := b.integrationRepo.GetInstalledIntegration(stackId.String(), pluginType)
	if err != nil {
		logger.Error("failed to get cross trade integration", zap.String("stackId", stackId.String()), zap.String("pluginType", pluginType), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Failed to get cross trade integration",
			Data:    nil,
		}, err
	}
	if crossTradeIntegration == nil {
		logger.Error("cross trade integration not found", zap.String("stackId", stackId.String()), zap.String("pluginType", pluginType))
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Cross trade integration not found",
			Data:    nil,
		}, nil
	}

	stackConfig := dtos.DeployThanosRequest{}
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	logPath := utils.GetLogPath(stack.ID, "deploy-new-l2-chain")

	var deployNewL2ChainDeployment *entities.DeploymentEntity
	var integration *entities.IntegrationEntity
	defer func() {
		if r := recover(); r != nil {
			if deployNewL2ChainDeployment != nil {
				_ = b.deploymentRepo.UpdateDeploymentStatus(deployNewL2ChainDeployment.ID.String(), entities.DeploymentRunStatusFailed)
			}
			if integration != nil {
				_ = b.integrationRepo.UpdateIntegrationStatusWithReason(integration.ID.String(), entities.DeploymentStatusFailed, fmt.Sprint(r))
			}
		}
	}()
	var step string
	if mode == string(thanosConstants.CrossTradeDeployModeL2ToL1) {
		step = constants.DeployNewL2ChainL2L1Step
	} else if mode == string(thanosConstants.CrossTradeDeployModeL2ToL2) {
		step = constants.DeployNewL2ChainL2L2Step
	} else {
		logger.Error("invalid cross trade mode", zap.String("mode", mode))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Invalid cross trade mode",
			Data:    nil,
		}, errors.New("invalid cross trade mode")
	}
	deployNewL2ChainDeployment = &entities.DeploymentEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Step:    step,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  []byte("{}"),
	}
	if err := b.deploymentRepo.CreateDeployment(deployNewL2ChainDeployment); err != nil {
		logger.Error("failed to create deploy new L2 chain deployment record", zap.String("mode", mode), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Failed to create deployment record",
			Data:    nil,
		}, err
	}
	crossTradeMode := thanosConstants.CrossTradeDeployMode(mode)
	b.taskManager.AddTask(fmt.Sprintf("deploy-new-l2-chain-%s-%s", stackId.String(), mode), func(ctx context.Context) {
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
			logger.Error("failed to create thanos sdk client", zap.String("stackId", stackId.String()), zap.Error(err))
			_ = b.deploymentRepo.UpdateDeploymentStatus(deployNewL2ChainDeployment.ID.String(), entities.DeploymentRunStatusFailed)
			return
		}
		output, err := thanos.DeployNewL2Chain(ctx, sdkClient, crossTradeMode, request.L2ChainConfig)
		if err != nil {
			logger.Error("failed to deploy new L2 chain", zap.String("stackId", stackId.String()), zap.String("mode", mode), zap.Error(err))
			_ = b.deploymentRepo.UpdateDeploymentStatus(deployNewL2ChainDeployment.ID.String(), entities.DeploymentRunStatusFailed)
			return
		}

		// Update integration metadata
		metadata := map[string]interface{}{
			"contracts":         output.DeployCrossTradeContractsOutput,
			"url":               output.DeployCrossTradeApplicationOutput.URL,
			"registered_tokens": output.RegisterTokens,
		}
		metadataBytes, err := json.Marshal(metadata)
		if err != nil {
			logger.Error("failed to marshal cross trade integration metadata", zap.String("stackId", stackId.String()), zap.String("pluginType", pluginType), zap.Error(err))
			return
		}
		if err := b.integrationRepo.UpdateMetadataAfterInstalled(crossTradeIntegration.ID.String(), entities.IntegrationInfo(metadataBytes)); err != nil {
			logger.Error("failed to update cross trade integration metadata", zap.String("stackId", stackId.String()), zap.String("pluginType", pluginType), zap.Error(err))
			return
		}

		var config dtos.InstallCrossChainBridgeRequest
		if err := json.Unmarshal(crossTradeIntegration.Config, &config); err != nil {
			logger.Error("failed to unmarshal cross trade integration config", zap.String("stackId", stackId.String()), zap.String("pluginType", pluginType), zap.Error(err))
			return
		}
		config.L2ChainConfig = append(config.L2ChainConfig, request.L2ChainConfig)
		configBytes, err := json.Marshal(config)
		if err != nil {
			logger.Error("failed to marshal cross trade integration config", zap.String("stackId", stackId.String()), zap.String("pluginType", pluginType), zap.Error(err))
			return
		}
		if err := b.integrationRepo.UpdateConfig(crossTradeIntegration.ID.String(), configBytes); err != nil {
			logger.Error("failed to update cross trade integration config", zap.String("stackId", stackId.String()), zap.String("pluginType", pluginType), zap.Error(err))
			return
		}

		_ = b.deploymentRepo.UpdateDeploymentStatus(deployNewL2ChainDeployment.ID.String(), entities.DeploymentRunStatusSuccess)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (b *CrossTradeBridgeIntegration) Cancel(ctx context.Context, stackId uuid.UUID, integrationId uuid.UUID) (*entities.Response, error) {
	// 1 Fetch integration
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

	var mode string
	if integration.Type == enum.IntegrationTypeCrossTradeL2ToL1.String() {
		mode = string(thanosConstants.CrossTradeDeployModeL2ToL1)
	} else if integration.Type == enum.IntegrationTypeCrossTradeL2ToL2.String() {
		mode = string(thanosConstants.CrossTradeDeployModeL2ToL2)
	} else {
		logger.Error("invalid cross trade mode", zap.String("mode", integration.Type))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Invalid cross trade mode",
			Data:    nil,
		}, errors.New("invalid cross trade mode")
	}

	// 2 validate status: can only cancel if inprogress or pending
	if integration.Status != string(entities.DeploymentStatusInProgress) && integration.Status != string(entities.DeploymentStatusPending) {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Can only cancel installations that are in progress or pending",
			Data:    nil,
		}, nil
	}

	// Set the integration status to cancelling
	if err = b.integrationRepo.UpdateIntegrationStatusWithReason(
		integration.ID.String(),
		entities.DeploymentStatusCancelling,
		"Stopping installation process. This may take several minutes to safely clean up AWS resources (RDS database, networking).",
	); err != nil {
		logger.Error("failed to request cancellation", zap.Error(err), zap.String("integrationId", integrationId.String()))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Failed to request cancellation",
			Data:    nil,
		}, err
	}

	b.taskManager.AddTask(fmt.Sprintf("cancel-cross-trade-%s", stackId.String()), func(ctx context.Context) {
		taskId := fmt.Sprintf("install-%s-%s", integration.Type, stackId)
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
			utils.GetLogPath(stack.ID, fmt.Sprintf("cancel-cross-trade-%s", mode)),
			string(stack.Network),
			stack.DeploymentPath,
			stackConfig.RegisterCandidate,
			stackConfig.AwsAccessKey,
			stackConfig.AwsSecretAccessKey,
			stackConfig.AwsRegion,
		)

		// Clean up any resources that were created
		if cleanupErr := thanos.UninstallCrossTradeBridge(ctx, sdkClient, mode); cleanupErr != nil {
			logger.Error("failed to cleanup resources during cancellation", zap.Error(cleanupErr))

			_ = b.integrationRepo.UpdateIntegrationStatusWithReason(
				integration.ID.String(),
				entities.DeploymentStatusFailed,
				cleanupErr.Error(),
			)
			return
		}

		logger.Info("Cancellation requested successfully", zap.String("integrationId", integrationId.String()))
		_ = b.integrationRepo.UpdateIntegrationStatusWithReason(
			integration.ID.String(),
			entities.DeploymentStatusCancelled,
			"Installation cancelled successfully. All AWS resources have been cleaned up.",
		)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Cancellation in progress. Installation will be stopped and AWS resources will be cleaned up. This may take 5-10 minutes for safe cleanup.",
		Data:    nil,
	}, nil
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
