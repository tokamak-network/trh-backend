package thanos

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"go.uber.org/zap"
)

func (s *ThanosStackDeploymentService) GetAllStacks(c *gin.Context) (*entities.Response, error) {
	userIDStr, exists := c.Get("user_id")
	var userID uuid.UUID
	if exists {
		userID, _ = uuid.Parse(userIDStr.(string))
	}
	stacks, err := s.stackRepo.GetAllStacks()
	if err != nil {
		logger.Error("failed to get stacks", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusInternalServerError,
			Message: "Internal server error",
			Data:    nil,
		}, err
	}
	// Filter stacks by deployer_id
	userStacks := make([]*entities.StackEntity, 0)
	for _, stack := range stacks {
		if stack.DeployerID == userID {
			userStacks = append(userStacks, stack)
		}
	}
	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"stacks": userStacks},
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

func (s *ThanosStackDeploymentService) GetDeployments(c *gin.Context, stackId uuid.UUID) (*entities.Response, error) {
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
	userIDStr, exists := c.Get("user_id")
	if exists {
		userID, _ := uuid.Parse(userIDStr.(string))
		if stack.DeployerID != uuid.Nil && userID != uuid.Nil && stack.DeployerID != userID {
			return &entities.Response{
				Status:  http.StatusForbidden,
				Message: "You are not authorized to access this stack's deployments.",
				Data:    nil,
			}, nil
		}
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

func (s *ThanosStackDeploymentService) GetStackDeploymentStatus(c *gin.Context, stackId uuid.UUID, deploymentId uuid.UUID) (*entities.Response, error) {
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
	userIDStr, exists := c.Get("user_id")
	if exists {
		userID, _ := uuid.Parse(userIDStr.(string))
		if stack.DeployerID != uuid.Nil && userID != uuid.Nil && stack.DeployerID != userID {
			return &entities.Response{
				Status:  http.StatusForbidden,
				Message: "You are not authorized to access this stack's deployments.",
				Data:    nil,
			}, nil
		}
	}
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

func (s *ThanosStackDeploymentService) GetStackDeployment(c *gin.Context, stackId uuid.UUID, deploymentId uuid.UUID) (*entities.Response, error) {
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
	userIDStr, exists := c.Get("user_id")
	if exists {
		userID, _ := uuid.Parse(userIDStr.(string))
		if stack.DeployerID != uuid.Nil && userID != uuid.Nil && stack.DeployerID != userID {
			return &entities.Response{
				Status:  http.StatusForbidden,
				Message: "You are not authorized to access this stack's deployments.",
				Data:    nil,
			}, nil
		}
	}
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

func (s *ThanosStackDeploymentService) GetStackByID(
	c *gin.Context,
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

	// Secure: Only deployer can access
	userIDStr, exists := c.Get("user_id")
	if exists {
		userID, _ := uuid.Parse(userIDStr.(string))
		if stack.DeployerID != uuid.Nil && userID != uuid.Nil && stack.DeployerID != userID {
			return &entities.Response{
				Status:  http.StatusForbidden,
				Message: "You are not authorized to access this stack.",
				Data:    nil,
			}, nil
		}
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Successfully",
		Data:    map[string]interface{}{"stack": stack},
	}, nil
}

func (s *ThanosStackDeploymentService) GetIntegrations(c *gin.Context, stackId uuid.UUID) (*entities.Response, error) {
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
	userIDStr, exists := c.Get("user_id")
	if exists {
		userID, _ := uuid.Parse(userIDStr.(string))
		if stack.DeployerID != uuid.Nil && userID != uuid.Nil && stack.DeployerID != userID {
			return &entities.Response{
				Status:  http.StatusForbidden,
				Message: "You are not authorized to access this stack's integrations.",
				Data:    nil,
			}, nil
		}
	}
	integrations, err := s.integrationRepo.GetActiveIntegrationsByStackID(stackId.String())
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

func (s *ThanosStackDeploymentService) GetIntegration(c *gin.Context, stackId uuid.UUID, integrationId uuid.UUID) (*entities.Response, error) {
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
	userIDStr, exists := c.Get("user_id")
	if exists {
		userID, _ := uuid.Parse(userIDStr.(string))
		if stack.DeployerID != uuid.Nil && userID != uuid.Nil && stack.DeployerID != userID {
			return &entities.Response{
				Status:  http.StatusForbidden,
				Message: "You are not authorized to access this stack's integrations.",
				Data:    nil,
			}, nil
		}
	}
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
