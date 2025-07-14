package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/enum"
	"github.com/tokamak-network/trh-backend/pkg/stacks/thanos"
	thanosStack "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
	"go.uber.org/zap"
)

// RegisterCandidateIntegration handles register candidate operations
type RegisterCandidateIntegration struct {
	stackRepo interface {
		GetStackByID(id string) (*entities.StackEntity, error)
	}
	integrationRepo interface {
		GetActiveIntegrations(stackId, integrationType string) ([]*entities.IntegrationEntity, error)
		CreateIntegration(integration *entities.IntegrationEntity) error
		UpdateIntegrationStatus(id string, status entities.DeploymentStatus) error
		UpdateIntegrationStatusWithReason(id string, status entities.DeploymentStatus, reason string) error
		UpdateMetadataAfterInstalled(id string, metadata entities.IntegrationInfo) error
	}
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
	}
}

// NewRegisterCandidateIntegration creates a new register candidate integration handler
func NewRegisterCandidateIntegration(
	stackRepo interface {
		GetStackByID(id string) (*entities.StackEntity, error)
	},
	integrationRepo interface {
		GetActiveIntegrations(stackId, integrationType string) ([]*entities.IntegrationEntity, error)
		CreateIntegration(integration *entities.IntegrationEntity) error
		UpdateIntegrationStatus(id string, status entities.DeploymentStatus) error
		UpdateIntegrationStatusWithReason(id string, status entities.DeploymentStatus, reason string) error
		UpdateMetadataAfterInstalled(id string, metadata entities.IntegrationInfo) error
	},
	taskManager interface {
		AddTask(id string, task func(ctx context.Context))
	},
) *RegisterCandidateIntegration {
	return &RegisterCandidateIntegration{
		stackRepo:       stackRepo,
		integrationRepo: integrationRepo,
		taskManager:     taskManager,
	}
}

// Register registers a candidate for the given stack
func (r *RegisterCandidateIntegration) Register(ctx context.Context, stackId uuid.UUID, req dtos.RegisterCandidateRequest) (*entities.Response, error) {
	stack, err := r.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error("failed to get stack by id", zap.Error(err))
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

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack has not been deployed yet",
			Data:    nil,
		}, nil
	}

	// check if register candidate is already in non-terminated state
	integrations, err := r.integrationRepo.GetActiveIntegrations(stackId.String(), enum.IntegrationTypeRegisterCandidate.String())
	if err != nil {
		logger.Error("failed to get integration", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if len(integrations) > 0 {
		logger.Error("There is already an active register candidate", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "There is already an active register candidate",
			Data:    nil,
		}, nil
	}

	stackConfig := dtos.DeployThanosRequest{}
	err = json.Unmarshal(stack.Config, &stackConfig)
	if err != nil {
		logger.Error("failed to unmarshal stack config", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	registerCandidateLogPath := utils.GetLogPath(stackId, "register-candidate")
	sdkClient, err := thanos.NewThanosSDKClient(
		ctx,
		registerCandidateLogPath,
		string(stack.Network),
		stack.DeploymentPath,
		stackConfig.RegisterCandidate,
		stackConfig.AwsAccessKey,
		stackConfig.AwsSecretAccessKey,
		stackConfig.AwsRegion,
	)
	if err != nil {
		logger.Error("failed to create thanos sdk client", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("register-candidate-%s", stackId.String())
	r.taskManager.AddTask(taskId, func(ctx context.Context) {
		r.registerTask(ctx, stack, sdkClient, req, registerCandidateLogPath, stackId)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Candidate registered successfully",
		Data:    nil,
	}, nil
}

// registerTask handles the actual registration process
func (r *RegisterCandidateIntegration) registerTask(ctx context.Context, stack *entities.StackEntity, sdkClient interface{}, req dtos.RegisterCandidateRequest, logPath string, stackId uuid.UUID) {
	integrationConfig, err := json.Marshal(req)
	if err != nil {
		logger.Error("failed to marshal integration config", zap.Error(err))
		return
	}

	integrationId := uuid.New()
	integration := &entities.IntegrationEntity{
		ID:      integrationId,
		StackID: &stackId,
		Type:    enum.IntegrationTypeRegisterCandidate.String(),
		Status:  string(entities.DeploymentStatusPending),
		Config:  integrationConfig,
		LogPath: logPath,
	}

	if err = r.integrationRepo.CreateIntegration(integration); err != nil {
		logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()), zap.Error(err))
		return
	}

	thanosClient, ok := sdkClient.(*thanosStack.ThanosStack)
	if !ok {
		logger.Error("failed to type assert sdkClient", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()))
		if updateErr := r.integrationRepo.UpdateIntegrationStatusWithReason(integrationId.String(), entities.DeploymentStatusFailed, "Invalid SDK client type"); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()), zap.Error(updateErr), zap.String("integrationId", integrationId.String()))
		}
		return
	}

	if err = thanos.VerifyRegisterCandidates(ctx, thanosClient, &req); err != nil {
		logger.Error("failed to register candidate", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()), zap.Error(err), zap.String("stackId", stackId.String()))
		if updateErr := r.integrationRepo.UpdateIntegrationStatusWithReason(integrationId.String(), entities.DeploymentStatusFailed, err.Error()); updateErr != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()), zap.Error(updateErr), zap.String("integrationId", integrationId.String()))
		}
		return
	}

	if err = r.integrationRepo.UpdateIntegrationStatus(integrationId.String(), entities.DeploymentStatusCompleted); err != nil {
		logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()), zap.Error(err), zap.String("integrationId", integrationId.String()))
	}

	registerCandidateInfo, err := thanos.GetRegisterCandidatesInfo(ctx, thanosClient, &req)
	if err != nil {
		logger.Error("failed to get register candidate info", zap.Error(err))
		return
	}

	bytes, err := json.Marshal(registerCandidateInfo)
	if err != nil {
		logger.Error("failed to marshal register candidate info", zap.Error(err))
		return
	}

	if err = r.integrationRepo.UpdateMetadataAfterInstalled(integrationId.String(), bytes); err != nil {
		logger.Error("failed to update register candidate integration metadata", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()), zap.Error(err))
		return
	}

	logger.Info("Register candidate successfully", zap.String("stackId", stackId.String()))
}
