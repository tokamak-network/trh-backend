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
	},
	logRepo interface {
		CreateLog(log *entities.LogEntity) error
	},
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
	},
) *IntegrationManager {
	return &IntegrationManager{
		blockExplorer:       NewBlockExplorerIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		bridge:              NewBridgeIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		monitoring:          NewMonitoringIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		registerCandidate:   NewRegisterCandidateIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
		registerMetadataDAO: NewRegisterMetadataDAOIntegration(stackRepo, deploymentRepo, integrationRepo, logRepo, taskManager),
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
