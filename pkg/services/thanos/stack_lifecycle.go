package thanos

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
	"go.uber.org/zap"
)

func (s *ThanosStackDeploymentService) CreateThanosStack(
	ctx context.Context,
	request dtos.DeployThanosRequest,
) (*entities.Response, error) {
	stackId := uuid.New()
	deploymentPath := utils.GetDeploymentPath(s.name, request.Network, stackId.String())
	request.DeploymentPath = deploymentPath
	config, err := json.Marshal(request)
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}
	stack := &entities.StackEntity{
		ID:             stackId,
		Name:           s.name,
		Network:        request.Network,
		Type:           enum.StackTypeOptimisticRollup.String(),
		Config:         config,
		DeploymentPath: deploymentPath,
		Status:         entities.StackStatusPending,
	}

	// We install the bridge by default
	integrations := make([]*entities.IntegrationEntity, 0)
	bridgeIntegration := &entities.IntegrationEntity{
		ID:      uuid.New(),
		StackID: &stack.ID,
		Type:    enum.IntegrationTypeBridge.String(),
		Status:  string(entities.DeploymentStatusPending),
	}
	integrations = append(integrations, bridgeIntegration)

	if request.RegisterCandidate {
		registerCandidateIntegration := &entities.IntegrationEntity{
			ID:      uuid.New(),
			StackID: &stack.ID,
			Type:    enum.IntegrationTypeRegisterCandidate.String(),
			Status:  string(entities.DeploymentStatusPending),
		}
		integrations = append(integrations, registerCandidateIntegration)
	}

	deployments, err := s.getThanosStackDeployments(stackId, &request)
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	err = s.stackRepo.CreateStackByTx(stack, deployments, integrations)
	if err != nil {
		logger.Error("Failed to create thanos stack", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	logger.Info("Stack created", zap.String("stackId", stackId.String()))

	taskId := fmt.Sprintf("deploy-thanos-stack-%s", stackId.String())
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		s.deploy(ctx, stackId)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]string{"stackId": stackId.String()},
	}, nil
}

func (s *ThanosStackDeploymentService) StopDeployingThanosStack(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
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

	if stack.Status != entities.StackStatusDeploying {
		if stack.Status == entities.StackStatusDeployed {
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: "Stack is already deployed, if you want to stop it, please use the terminate it",
				Data:    nil,
			}, nil
		}
		if stack.Status == entities.StackStatusTerminating {
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: "Stack is terminating, please wait for it to finish",
				Data:    nil,
			}, nil
		}

		if stack.Status == entities.StackStatusTerminated {
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: "Stack is terminated, you cannot stop it",
				Data:    nil,
			}, nil
		}

		if stack.Status == entities.StackStatusFailedToDeploy {
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: "Stack failed to deploy, you cannot stop it",
				Data:    nil,
			}, nil
		}

		if stack.Status == entities.StackStatusFailedToTerminate {
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: "Stack failed to terminate, you cannot stop it",
				Data:    nil,
			}, nil
		}

		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deploying, yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}

	taskId := fmt.Sprintf("deploy-thanos-stack-%s", stackId.String())
	s.taskManager.StopTask(taskId)
	// Update stacks status to stopping
	err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusStopped, "")
	if err != nil {
		logger.Error("failed to update stacks status",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}
	err = s.deploymentRepo.UpdateStatusesByStackId(stackId.String(), entities.DeploymentRunStatusStopped)
	if err != nil {
		logger.Error("failed to update deployment status",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
	}

	err = s.integrationRepo.UpdateIntegrationStatusByStackID(stackId.String(), entities.DeploymentStatusStopped)
	if err != nil {
		logger.Error("failed to update integration status",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (s *ThanosStackDeploymentService) ResumeThanosStack(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
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

	if stack.Status != entities.StackStatusStopped &&
		stack.Status != entities.StackStatusFailedToDeploy && stack.Status != entities.StackStatusTerminated {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not stopped, yet. Please wait for it to finish",
			Data:    nil,
		}, nil
	}
	// Create fresh deployment records for this resume
	var stackConfig dtos.DeployThanosRequest
	if err := json.Unmarshal(stack.Config, &stackConfig); err != nil {
		logger.Error("failed to unmarshal stack config", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	// Get stopped pendingDeployments
	pendingDeployments, err := s.getThanosStackDeployments(stackId, &stackConfig)
	if err != nil {
		logger.Error("failed to get deployments", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	for _, d := range pendingDeployments {
		err = s.deploymentRepo.CreateDeployment(d)
		if err != nil {
			logger.Error("failed to create deployment record on resume", zap.String("stackId", stackId.String()), zap.Error(err))
			return &entities.Response{
				Status:  http.StatusInternalServerError,
				Message: "Internal server error",
				Data:    nil,
			}, err
		}
	}

	err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusPending, "")
	if err != nil {
		logger.Error("failed to update stack status to pending", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("deploy-thanos-stack-%s", stackId.String())
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		s.deploy(ctx, stackId)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (s *ThanosStackDeploymentService) UpdateNetwork(ctx context.Context, stackId uuid.UUID, request dtos.UpdateNetworkRequest) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
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

	if stack.Status != entities.StackStatusDeployed {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "Stack is not deployed, yet. Please wait for it to finish",
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

	logPath := utils.GetLogPath(stack.ID, "update-network")
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
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusUpdating, "")
	if err != nil {
		logger.Error("failed to update stack status", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("update-network-%s", stackId.String())
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		err = thanos.UpdateNetwork(ctx, sdkClient, &request)
		if err != nil {
			logger.Error("failed to update network", zap.Error(err))
			return
		}

		// Update stack config with new L1RPC and L1Beacon URLs
		stackConfig.L1RpcUrl = request.L1RpcUrl
		stackConfig.L1BeaconUrl = request.L1BeaconUrl

		updatedConfig, err := json.Marshal(stackConfig)
		if err != nil {
			logger.Error("failed to marshal updated stack config", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}

		err = s.stackRepo.UpdateConfig(stackId.String(), updatedConfig)
		if err != nil {
			logger.Error("failed to update stack config", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}

		err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusDeployed, "")
		if err != nil {
			logger.Error("failed to update stack status", zap.String("stackId", stackId.String()), zap.Error(err))
			return
		}
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}

func (s *ThanosStackDeploymentService) TerminateThanosStack(ctx context.Context, stackId uuid.UUID) (*entities.Response, error) {
	// Check if stacks exists
	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	// Check if stacks is in a valid state to be terminated
	if stack.Status == entities.StackStatusDeploying || stack.Status == entities.StackStatusUpdating ||
		stack.Status == entities.StackStatusTerminating {
		logger.Error(
			"The stacks is still deploying, updating or terminating, please wait for it to finish",
			zap.String("stackId", stackId.String()),
		)
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "The stacks is still deploying, updating or terminating, please wait for it to finish",
			Data:    nil,
		}, nil
	}

	// Update stack status to pending termination
	err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusPending, "")
	if err != nil {
		logger.Error("failed to update stack status to pending termination",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Failed to update stack status to pending termination",
			Data:    nil,
		}, err
	}

	taskId := fmt.Sprintf("terminate-thanos-stack-%s", stackId.String())
	s.taskManager.AddTask(taskId, func(ctx context.Context) {
		s.handleStackTermination(ctx, stack)
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    nil,
	}, nil
}
