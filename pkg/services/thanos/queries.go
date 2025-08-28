package thanos

import (
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
