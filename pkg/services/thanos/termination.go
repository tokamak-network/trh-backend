package thanos

import (
	"context"
	"encoding/json"

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

func (s *ThanosStackDeploymentService) handleStackTermination(ctx context.Context, stack *entities.StackEntity) {
	// Check if stacks exists
	if stack == nil {
		logger.Error("stack not found")
		return
	}

	stackId := stack.ID

	stackConfig := dtos.DeployThanosRequest{}
	err := json.Unmarshal(stack.Config, &stackConfig)
	if err != nil {
		logger.Error("failed to unmarshal stacks config",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		if updateErr := s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusFailedToTerminate, err.Error()); updateErr != nil {
			logger.Error("failed to update stacks status after unmarshal error",
				zap.String("stackId", stackId.String()),
				zap.Error(updateErr))
		}
		return
	}

	logPath := utils.GetLogPath(stack.ID, "destroy")

	// Create a deployment record for termination
	terminationDeploymentID := uuid.New()
	terminationDeployment := &entities.DeploymentEntity{
		ID:      terminationDeploymentID,
		StackID: &stack.ID,
		Step:    constants.DestroyChainStep,
		Status:  entities.DeploymentRunStatusInProgress,
		LogPath: logPath,
		Config:  nil,
	}
	if err := s.deploymentRepo.CreateDeployment(terminationDeployment); err != nil {
		logger.Error("failed to create termination deployment",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

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
		logger.Error("failed to create thanos sdk client",
			zap.Error(err))
		return
	}

	err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusTerminating, "")
	if err != nil {
		logger.Error("failed to update stacks status after destroy error",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	err = s.integrationRepo.UpdateIntegrationsStatusByStackID(
		stackId.String(),
		entities.DeploymentStatusTerminating,
		[]entities.DeploymentStatus{entities.DeploymentStatusTerminated},
		[]string{enum.IntegrationTypeRegisterCandidate.String(), enum.IntegrationTypeRegisterMetadataDAO.String()},
	)
	if err != nil {
		logger.Error("failed to update integrations status to terminating",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	// Start log ingestion for termination
	ingestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go s.tailAndIngestDeploymentLogs(ingestCtx, stack.ID, terminationDeploymentID, logPath)

	err = thanos.DestroyAWSInfrastructure(ctx, sdkClient)
	if err != nil {
		logger.Error("failed to destroy AWS infrastructure",
			zap.String("stackId", stackId.String()),
			zap.Error(err))

		updateErr := s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusFailedToTerminate, err.Error())
		if updateErr != nil {
			logger.Error("failed to update stacks status after destroy error",
				zap.String("stackId", stackId.String()),
				zap.Error(updateErr))
		}
		_ = s.deploymentRepo.UpdateDeploymentStatus(terminationDeploymentID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusTerminated, "")
	if err != nil {
		logger.Error("failed to update stacks status to terminated",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		_ = s.deploymentRepo.UpdateDeploymentStatus(terminationDeploymentID.String(), entities.DeploymentRunStatusFailed)
		return
	}

	// Update integrations status to terminated
	err = s.integrationRepo.UpdateIntegrationsStatusByStackID(
		stackId.String(),
		entities.DeploymentStatusTerminated,
		[]entities.DeploymentStatus{},
		[]string{enum.IntegrationTypeRegisterCandidate.String(), enum.IntegrationTypeRegisterMetadataDAO.String()},
	)
	if err != nil {
		logger.Error("failed to update integrations status to terminated",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		_ = s.deploymentRepo.UpdateDeploymentStatus(terminationDeploymentID.String(), entities.DeploymentRunStatusFailed)
		return
	}
	_ = s.deploymentRepo.UpdateDeploymentStatus(terminationDeploymentID.String(), entities.DeploymentRunStatusSuccess)

	logger.Info(
		"AWS infrastructure destroyed successfully",
		zap.String("stackId", stackId.String()),
	)
}
