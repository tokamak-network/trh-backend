package integrations

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"go.uber.org/zap"
)

// IntegrationManager provides a unified interface for managing all integrations
type IntegrationManager struct {
	blockExplorer       *BlockExplorerIntegration
	bridge              *BridgeIntegration
	monitoring          *MonitoringIntegration
	registerCandidate   *RegisterCandidateIntegration
	registerMetadataDAO *RegisterMetadataDAOIntegration
	backupManager       *BackupManager
	crossTrade          *CrossTradeBridgeIntegration
	uptimeService       *UptimeServiceIntegration
	drb                 *DRBIntegration
}

// NewIntegrationManager creates a new integration manager with all integration handlers
func NewIntegrationManager(
	stackRepo interface {
		GetStackByID(id string) (*entities.StackEntity, error)
		UpdateMetadata(id string, metadata *entities.StackMetadata) error
	},
	deploymentRepo interface {
		CreateDeployment(deployment *entities.DeploymentEntity) error
		UpdateDeploymentStatus(deploymentId string, status entities.DeploymentRunStatus) error
		GetDeploymentByStepAndStatus(stackID string, step string, status entities.DeploymentStatus) (*entities.DeploymentEntity, error)
	},
	integrationRepo interface {
		GetActiveIntegrations(stackId, integrationType string) ([]*entities.IntegrationEntity, error)
		CreateIntegration(integration *entities.IntegrationEntity) error
		UpdateIntegrationStatus(id string, status entities.DeploymentStatus) error
		UpdateIntegrationStatusWithReason(id string, status entities.DeploymentStatus, reason string) error
		GetInstalledIntegration(stackId, integrationType string) (*entities.IntegrationEntity, error)
		UpdateConfig(id string, config json.RawMessage) error
		UpdateMetadataAfterInstalled(id string, metadata entities.IntegrationInfo) error
		GetIntegrationByStatus(stackId string, integrationType string, status entities.DeploymentStatus) (*entities.IntegrationEntity, error)
		GetIntegrationById(id string) (*entities.IntegrationEntity, error)
	},
	logRepo interface {
		CreateLog(log *entities.LogEntity) error
	},
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
		AddTaskWithProgress(id string, task func(ctx context.Context, updateProgress func(string, float64)))
		SetTaskResult(id string, result any)
		StopTask(id string)
		IsTaskRunning(id string) bool
	},
) *IntegrationManager {
	return &IntegrationManager{
		blockExplorer:       NewBlockExplorerIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		bridge:              NewBridgeIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		monitoring:          NewMonitoringIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		registerCandidate:   NewRegisterCandidateIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		registerMetadataDAO: NewRegisterMetadataDAOIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		backupManager:       NewBackupManager(stackRepo, deploymentRepo, integrationRepo, taskManager),
		crossTrade:          NewCrossTradeBridgeIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		uptimeService:       NewUptimeServiceIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		drb:                 NewDRBIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
	}
}

// BlockExplorer returns the block explorer integration handler
func (im *IntegrationManager) BlockExplorer() *BlockExplorerIntegration {
	return im.blockExplorer
}

// Bridge returns the bridge integration handler
func (im *IntegrationManager) Bridge() *BridgeIntegration {
	return im.bridge
}

// Monitoring returns the monitoring integration handler
func (im *IntegrationManager) Monitoring() *MonitoringIntegration {
	return im.monitoring
}

// RegisterCandidate returns the register candidate integration handler
func (im *IntegrationManager) RegisterCandidate() *RegisterCandidateIntegration {
	return im.registerCandidate
}

// InstallBlockExplorer installs a block explorer for the given stack
func (im *IntegrationManager) InstallBlockExplorer(ctx context.Context, stackId string, request dtos.InstallBlockExplorerRequest) (*entities.Response, error) {
	return im.blockExplorer.Install(ctx, stackId, request)
}

// UninstallBlockExplorer uninstalls the block explorer for the given stack
func (im *IntegrationManager) UninstallBlockExplorer(ctx context.Context, stackId string) (*entities.Response, error) {
	return im.blockExplorer.Uninstall(ctx, stackId)
}

// InstallBridge installs a bridge for the given stack
func (im *IntegrationManager) InstallBridge(ctx context.Context, stackId string) (*entities.Response, error) {
	return im.bridge.Install(ctx, stackId)
}

// UninstallBridge uninstalls the bridge for the given stack
func (im *IntegrationManager) UninstallBridge(ctx context.Context, stackId string) (*entities.Response, error) {
	return im.bridge.Uninstall(ctx, stackId)
}

// InstallMonitoring installs monitoring for the given stack
func (im *IntegrationManager) InstallMonitoring(ctx context.Context, stackId uuid.UUID, req dtos.InstallMonitoringRequest) (*entities.Response, error) {
	return im.monitoring.Install(ctx, stackId, req)
}

// UninstallMonitoring uninstalls the monitoring for the given stack
func (im *IntegrationManager) UninstallMonitoring(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	return im.monitoring.Uninstall(ctx, stackId)
}

// RegisterCandidateForStack registers a candidate for the given stack
func (im *IntegrationManager) RegisterCandidateForStack(ctx context.Context, stackId uuid.UUID, req dtos.RegisterCandidateRequest) (*entities.Response, error) {
	return im.registerCandidate.Register(ctx, stackId, req)
}

func (im *IntegrationManager) RegisterMetadataDAOForStack(ctx context.Context, stackId uuid.UUID, req dtos.RegisterMetadataDAORequest) (*entities.Response, error) {
	return im.registerMetadataDAO.Register(ctx, stackId, req)
}

func (im *IntegrationManager) GetRegisterMetadataDAOForStack(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	return im.registerMetadataDAO.Get(ctx, stackId)
}

func (im *IntegrationManager) BackupStatus(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	return im.backupManager.GetBackupStatus(ctx, stackId)
}

func (im *IntegrationManager) BackupCheckpoints(ctx context.Context, stackId uuid.UUID, request dtos.BackupRequest) (*entities.Response, error) {
	return im.backupManager.GetCheckpoints(ctx, stackId, request)
}

func (im *IntegrationManager) BackupSnapshot(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	return im.backupManager.BackupSnapshot(ctx, stackId)
}

func (im *IntegrationManager) BackupRestore(ctx context.Context, stackId uuid.UUID, request dtos.BackupRestoreRequest) (*entities.Response, error) {
	return im.backupManager.BackupRestore(ctx, stackId, request)
}

func (im *IntegrationManager) BackupConfigure(ctx context.Context, stackId uuid.UUID, request dtos.BackupConfigureRequest) (*entities.Response, error) {
	return im.backupManager.BackupConfigure(ctx, stackId, request)
}

func (im *IntegrationManager) BackupAttach(ctx context.Context, stackId uuid.UUID, request dtos.BackupAttachRequest) (*entities.Response, error) {
	return im.backupManager.BackupAttach(ctx, stackId, request)
}

func (im *IntegrationManager) BackupPvPvcExport(ctx context.Context, stackId uuid.UUID) (string, string, error) {
	return im.backupManager.BackupPvPvcExport(ctx, stackId)
}

func (im *IntegrationManager) BackupCleanup(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	return im.backupManager.BackupCleanup(ctx, stackId)
}

// DisableEmailAlert disables email alert configuration for the given stack
func (im *IntegrationManager) DisableEmailAlert(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	return im.monitoring.DisableEmailAlert(ctx, stackId)
}

// DisableTelegramAlert disables telegram alert configuration for the given stack
func (im *IntegrationManager) DisableTelegramAlert(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	return im.monitoring.DisableTelegramAlert(ctx, stackId)
}

// UpdateEmailAlert updates email alert configuration for the given stack
func (im *IntegrationManager) UpdateEmailAlert(ctx context.Context, stackId uuid.UUID, request dtos.UpdateEmailConfigRequest) (*entities.Response, error) {
	return im.monitoring.UpdateEmailAlert(ctx, stackId, request)
}

// UpdateTelegramAlert updates telegram alert configuration for the given stack
func (im *IntegrationManager) UpdateTelegramAlert(ctx context.Context, stackId uuid.UUID, request dtos.UpdateTelegramConfigRequest) (*entities.Response, error) {
	return im.monitoring.UpdateTelegramAlert(ctx, stackId, request)
}

func (im *IntegrationManager) InstallCrossChainBridge(ctx context.Context, stackId uuid.UUID, request dtos.InstallCrossChainBridgeRequest) (*entities.Response, error) {
	return im.crossTrade.Install(ctx, stackId, request)
}

func (im *IntegrationManager) UninstallCrossChainBridge(ctx context.Context, stackId uuid.UUID, mode string) (*entities.Response, error) {
	return im.crossTrade.Uninstall(ctx, stackId.String(), mode)
}

func (im *IntegrationManager) RegisterTokens(ctx context.Context, stackId uuid.UUID, mode string, request dtos.RegisterTokensAPIRequest) (*entities.Response, error) {
	return im.crossTrade.RegisterTokens(ctx, stackId, mode, request)
}

func (im *IntegrationManager) DeployNewL2Chain(ctx context.Context, stackId uuid.UUID, mode string, request dtos.DeployNewL2ChainRequest) (*entities.Response, error) {
	return im.crossTrade.DeployNewL2Chain(ctx, stackId, mode, request)
}

func (im *IntegrationManager) AutoInstallCrossTradeAWS(ctx context.Context, stackId uuid.UUID, stackConfig *dtos.DeployThanosRequest, l2RPC string, l2ChainID uint64, l1ChainID uint64) {
	im.crossTrade.autoInstallCrossTradeAWS(ctx, stackId, stackConfig, l2RPC, l2ChainID, l1ChainID)
}

// InstallUptimeService installs an uptime service for the given stack
func (im *IntegrationManager) InstallUptimeService(ctx context.Context, stackId string) (*entities.Response, error) {
	return im.uptimeService.Install(ctx, stackId)
}

// UninstallUptimeService uninstalls the uptime service for the given stack
func (im *IntegrationManager) UninstallUptimeService(ctx context.Context, stackId string) (*entities.Response, error) {
	return im.uptimeService.Uninstall(ctx, stackId)
}

// UninstallDRB uninstalls the DRB integration for the given stack
func (im *IntegrationManager) UninstallDRB(ctx context.Context, stackId string) (*entities.Response, error) {
	return im.drb.Uninstall(ctx, stackId)
}

func (im *IntegrationManager) CancelIntegration(ctx context.Context, stackId uuid.UUID, integrationId uuid.UUID) (*entities.Response, error) {
	// fetch integration to see what type it is
	integration, err := im.blockExplorer.integrationRepo.GetIntegrationById(integrationId.String())
	if err != nil {
		return &entities.Response{
			Status:  500,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if integration == nil {
		return &entities.Response{
			Status:  404,
			Message: "Integration not found",
			Data:    nil,
		}, nil
	}

	// route to the right handler for this integration type
	switch integration.Type {
	case "block-explorer":
		return im.blockExplorer.Cancel(ctx, stackId, integrationId)
	case "bridge":
		return im.bridge.Cancel(ctx, stackId, integrationId)
	case "monitoring":
		return im.monitoring.Cancel(ctx, stackId, integrationId)
	case "system-pulse":
		return im.uptimeService.Cancel(ctx, stackId, integrationId)
	case "register-candidate":
		return im.registerCandidate.Cancel(ctx, stackId, integrationId)
	case "cross-trade":
		return im.crossTrade.Cancel(ctx, stackId, integrationId)
	default:
		return &entities.Response{
			Status:  400,
			Message: "Cancel not supported for this integration type",
			Data:    nil,
		}, nil
	}
}

func (im *IntegrationManager) RetryIntegration(ctx context.Context, stackId uuid.UUID, integrationId uuid.UUID) (*entities.Response, error) {
	// get integration to check its type
	integration, err := im.blockExplorer.integrationRepo.GetIntegrationById(integrationId.String())
	if err != nil {
		return &entities.Response{
			Status:  500,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if integration == nil {
		return &entities.Response{
			Status:  404,
			Message: "Integration not found",
			Data:    nil,
		}, nil
	}

	// route to the right handler
	switch integration.Type {
	case "block-explorer":
		return im.blockExplorer.Retry(ctx, stackId, integrationId)
	case "bridge":
		return im.bridge.Retry(ctx, stackId, integrationId)
	case "monitoring":
		return im.monitoring.Retry(ctx, stackId, integrationId)
	case "system-pulse":
		return im.uptimeService.Retry(ctx, stackId, integrationId)
	case "cross-trade":
		return im.crossTrade.Retry(ctx, stackId, integrationId)
	case "register-candidate":
		return &entities.Response{
			Status:  400,
			Message: "Cannot retry register candidate operations",
			Data:    nil,
		}, nil
	default:
		return &entities.Response{
			Status:  400,
			Message: "Retry not supported for this integration type",
			Data:    nil,
		}, nil
	}
}

// BadRequestError represents a client error (400) in integration operations
type BadRequestError struct {
	message string
}

func (e *BadRequestError) Error() string {
	return e.message
}

// implemented cancle individually per integration with sdk cleanup

// cancelIntegrationCommon implements common cancellation logic for all integration types
// func cancelIntegrationCommon(
// 	_ context.Context,
// 	stackId uuid.UUID,
// 	integrationId uuid.UUID,
// 	integrationRepo interface {
// 		GetIntegrationById(id string) (*entities.IntegrationEntity, error)
// 		UpdateIntegrationStatusWithReason(id string, status entities.DeploymentStatus, reason string) error
// 	},
// 	taskManager interface {
// 		StopTask(id string)
// 	},
// ) (*entities.Response, error) {
// 	integration, err := integrationRepo.GetIntegrationById(integrationId.String())
// 	if err != nil {
// 		logger.Error("failed to get integration", zap.Error(err), zap.String("integrationId", integrationId.String()))
// 		return &entities.Response{
// 			Status:  http.StatusInternalServerError,
// 			Message: "Internal server error",
// 			Data:    nil,
// 		}, err
// 	}
//
// 	if integration == nil {
// 		return &entities.Response{
// 			Status:  http.StatusNotFound,
// 			Message: "Integration not found",
// 			Data:    nil,
// 		}, nil
// 	}
//
// 	// can only cancel if its running or waiting to start
// 	if integration.Status != string(entities.DeploymentStatusInProgress) && integration.Status != string(entities.DeploymentStatusPending) {
// 		return &entities.Response{
// 			Status:  http.StatusBadRequest,
// 			Message: "Can only cancel installations that are in progress or pending",
// 			Data:    nil,
// 		}, nil
// 	}
//
// 	// stop the task
// 	taskId := fmt.Sprintf("install-%s-%s", integration.Type, stackId.String())
// 	taskManager.StopTask(taskId)
//
// 	if err := integrationRepo.UpdateIntegrationStatusWithReason(integration.ID.String(), entities.DeploymentStatusCancelled, "Cancelled by user"); err != nil {
// 		logger.Error("failed to update integration status", zap.Error(err), zap.String("integrationId", integrationId.String()))
// 		return &entities.Response{
// 			Status:  http.StatusInternalServerError,
// 			Message: "Internal server error",
// 			Data:    nil,
// 		}, err
// 	}
//
// 	return &entities.Response{
// 		Status:  http.StatusOK,
// 		Message: "Integration cancelled successfully",
// 		Data:    nil,
// 	}, nil
// }

// retryIntegrationCommon implements common retry logic for all integration types
func retryIntegrationCommon(
	_ context.Context,
	stackId uuid.UUID,
	integrationId uuid.UUID,
	integrationRepo interface {
		GetIntegrationById(id string) (*entities.IntegrationEntity, error)
		UpdateIntegrationStatus(id string, status entities.DeploymentStatus) error
	},
	stackRepo interface {
		GetStackByID(id string) (*entities.StackEntity, error)
	},
	retryCallback func(stack *entities.StackEntity, integration *entities.IntegrationEntity) error,
) (*entities.Response, error) {
	integration, err := integrationRepo.GetIntegrationById(integrationId.String())
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

	//  only allow retry for Cancelled not Failed
	// if integration.Status != string(entities.DeploymentStatusFailed) && integration.Status != string(entities.DeploymentStatusCancelled) {
	if integration.Status != string(entities.DeploymentStatusCancelled) {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Can only retry cancelled installations",
			Data:    nil,
		}, nil
	}

	stack, err := stackRepo.GetStackByID(stackId.String())
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

	if err := integrationRepo.UpdateIntegrationStatus(integration.ID.String(), entities.DeploymentStatusPending); err != nil {
		logger.Error("failed to update integration status", zap.Error(err), zap.String("integrationId", integrationId.String()))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	// invoke the callback to start the retry
	if err := retryCallback(stack, integration); err != nil {
		if _, ok := err.(*BadRequestError); ok {
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: err.Error(),
				Data:    nil,
			}, err
		}
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: err.Error(),
			Data:    nil,
		}, err
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Integration retry started successfully",
		Data:    nil,
	}, nil
}
