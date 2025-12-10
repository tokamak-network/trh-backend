package integrations

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
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
	},
) *IntegrationManager {
	return &IntegrationManager{
		blockExplorer:       NewBlockExplorerIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		bridge:              NewBridgeIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		monitoring:          NewMonitoringIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		registerCandidate:   NewRegisterCandidateIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		registerMetadataDAO: NewRegisterMetadataDAOIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		crossTrade:          NewCrossTradeBridgeIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		uptimeService:       NewUptimeServiceIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
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

func (im *IntegrationManager) UninstallCrossChainBridge(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	return im.crossTrade.Uninstall(ctx, stackId.String())
}

// InstallUptimeService installs an uptime service for the given stack
func (im *IntegrationManager) InstallUptimeService(ctx context.Context, stackId string) (*entities.Response, error) {
	return im.uptimeService.Install(ctx, stackId)
}

// UninstallUptimeService uninstalls the uptime service for the given stack
func (im *IntegrationManager) UninstallUptimeService(ctx context.Context, stackId string) (*entities.Response, error) {
	return im.uptimeService.Uninstall(ctx, stackId)
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
		return &entities.Response{
			Status:  400,
			Message: "Cannot cancel register candidate operations",
			Data:    nil,
		}, nil
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
