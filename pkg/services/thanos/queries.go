package thanos

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/enum"
	"go.uber.org/zap"
)

func (s *ThanosStackDeploymentService) GetAllStacks() (*entities.Response, error) {
	stacks, err := s.stackRepo.GetAllStacks()
	if err != nil {
		logger.Error("failed to get stacks", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"stacks": stacks},
	}, nil
}

func (s *ThanosStackDeploymentService) GetStackStatus(stackId uuid.UUID) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
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

	status, err := s.stackRepo.GetStackStatus(stackId.String())
	if err != nil {
		logger.Error("failed to get stack status", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"status": status},
	}, nil
}

func (s *ThanosStackDeploymentService) GetDeployments(
	stackId uuid.UUID,
) (*entities.Response, error) {

	stack, err := s.stackRepo.GetStackByID(stackId.String())
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

	deployments, err := s.deploymentRepo.GetDeploymentsByStackID(stackId.String())
	if err != nil {
		logger.Error("failed to get deployments", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"deployments": deployments},
	}, nil
}

func (s *ThanosStackDeploymentService) GetStackDeploymentStatus(
	deploymentId uuid.UUID,
) (*entities.Response, error) {
	status, err := s.deploymentRepo.GetDeploymentStatus(deploymentId.String())
	if err != nil {
		logger.Error("failed to get deployment status", zap.String("deploymentId", deploymentId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"status": status},
	}, nil
}

func (s *ThanosStackDeploymentService) GetStackDeployment(
	_ uuid.UUID,
	deploymentId uuid.UUID,
) (*entities.Response, error) {
	deployment, err := s.deploymentRepo.GetDeploymentByID(deploymentId.String())
	if err != nil {
		logger.Error("failed to get deployment", zap.String("deploymentId", deploymentId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	if deployment == nil {
		return &entities.Response{
			Status:  http.StatusNotFound,
			Message: "Deployment not found",
			Data:    nil,
		}, nil
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"deployment": deployment},
	}, nil
}

func (s *ThanosStackDeploymentService) GetDeploymentLogs(
	stackId uuid.UUID,
	deploymentId uuid.UUID,
	limit int,
	afterID *string,
) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error("failed to get stack", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{Status: http.StatusInternalServerError, Message: "Internal server error"}, err
	}
	if stack == nil {
		return &entities.Response{Status: http.StatusNotFound, Message: "Stack not found"}, nil
	}

	logs, err := s.logRepo.GetLogsByDeploymentID(deploymentId.String(), limit, afterID)
	if err != nil {
		logger.Error("failed to get logs", zap.String("deploymentId", deploymentId.String()), zap.Error(err))
		return &entities.Response{Status: http.StatusInternalServerError, Message: "Internal server error"}, err
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"logs": logs},
	}, nil
}

func (s *ThanosStackDeploymentService) GetStackLogs(
	stackId uuid.UUID,
	limit int,
	afterID *string,
) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
	if err != nil {
		logger.Error("failed to get stack", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{Status: http.StatusInternalServerError, Message: "Internal server error"}, err
	}
	if stack == nil {
		return &entities.Response{Status: http.StatusNotFound, Message: "Stack not found"}, nil
	}

	logs, err := s.logRepo.GetLogsByStackID(stackId.String(), limit, afterID)
	if err != nil {
		logger.Error("failed to get logs", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{Status: http.StatusInternalServerError, Message: "Internal server error"}, err
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"logs": logs},
	}, nil
}

func (s *ThanosStackDeploymentService) GetStackByID(
	stackId uuid.UUID,
) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
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

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"stack": stack},
	}, nil
}

func (s *ThanosStackDeploymentService) GetIntegrations(
	stackId uuid.UUID,
) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
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
	integrations, err := s.integrationRepo.GetActiveIntegrationsByStackID(stackId.String(), []string{enum.IntegrationTypeRegisterMetadataDAO.String()})
	if err != nil {
		logger.Error("failed to get integrations", zap.String("stackId", stackId.String()), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}
	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"integrations": integrations},
	}, nil
}

func (s *ThanosStackDeploymentService) GetIntegration(
	stackId uuid.UUID,
	integrationId uuid.UUID,
) (*entities.Response, error) {
	integration, err := s.integrationRepo.GetIntegrationById(integrationId.String())
	if err != nil {
		logger.Error("failed to get integrations", zap.String("stackId", stackId.String()), zap.Error(err))
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

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"integration": integration},
	}, nil
}

// Thanos Sepolia system stack configuration (Alpha release)
const (
	ThanosSepolia_ChainID     = 111551119090
	ThanosSepolia_Name        = "Thanos Sepolia (System)"
	ThanosSepolia_RpcUrl      = "https://rpc.thanos-sepolia.tokamak.network"
	ThanosSepolia_ExplorerUrl = "https://explorer.thanos-sepolia.tokamak.network"
)

// GetOrCreateThanosSepolia returns the Thanos Sepolia system stack, creating it if it doesn't exist
func (s *ThanosStackDeploymentService) GetOrCreateThanosSepolia() (*entities.Response, error) {
	// Try to find existing Thanos Sepolia stack by name
	stacks, err := s.stackRepo.GetAllStacks()
	if err != nil {
		logger.Error("failed to get stacks", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}

	// Look for existing Thanos Sepolia stack
	for _, stack := range stacks {
		if stack.Name == ThanosSepolia_Name {
			return &entities.Response{
				Status:  http.StatusOK,
				Message: "Successfully",
				Data:    map[string]interface{}{"stack": stack},
			}, nil
		}
	}

	// Create new Thanos Sepolia system stack
	stackId := uuid.New()
	metadata := &entities.StackMetadata{
		Layer1:      "Ethereum Sepolia",
		Layer2:      "Thanos Sepolia",
		L2RpcUrl:    ThanosSepolia_RpcUrl,
		L1ChainId:   11155111, // Sepolia
		L2ChainId:   int(ThanosSepolia_ChainID),
		ExplorerUrl: ThanosSepolia_ExplorerUrl,
	}

	stack := &entities.StackEntity{
		ID:       stackId,
		Name:     ThanosSepolia_Name,
		Type:     "thanos",
		Network:  entities.DeploymentNetworkTestnet,
		Config:   json.RawMessage(`{}`), // Empty config for system stack
		Metadata: metadata,
		Status:   entities.StackStatusDeployed, // Pre-deployed system stack
	}

	// Create system stack (no deployments or integrations needed)
	if err := s.stackRepo.CreateStack(stack); err != nil {
		logger.Error("failed to create Thanos Sepolia stack", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Failed to create system stack",
			Data:    nil,
		}, err
	}

	logger.Info("Created Thanos Sepolia system stack", zap.String("stackId", stackId.String()))

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully created Thanos Sepolia stack",
		Data:    map[string]interface{}{"stack": stack},
	}, nil
}
