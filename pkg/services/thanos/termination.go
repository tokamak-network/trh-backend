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
	terminationConfig, _ := json.Marshal(dtos.TerminateThanosRequest{
		Network:            string(stack.Network),
		AwsAccessKey:       stackConfig.AwsAccessKey,
		AwsSecretAccessKey: stackConfig.AwsSecretAccessKey,
		AwsRegion:          stackConfig.AwsRegion,
		DeploymentPath:     stack.DeploymentPath,
		LogPath:            logPath,
	})
	terminationDeployment := &entities.DeploymentEntity{
		ID:      terminationDeploymentID,
		StackID: &stack.ID,
		Step:    constants.DestroyChainStep,
		Status:  entities.DeploymentRunStatusNotStarted,
		LogPath: logPath,
		Config:  terminationConfig,
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

	err = s.integrationRepo.UpdateIntegrationsStatusByStackID(stackId.String(), entities.DeploymentStatusTerminating, []entities.DeploymentStatus{entities.DeploymentStatusTerminated})
	if err != nil {
		logger.Error("failed to update integrations status to terminating",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		return
	}

	// Start log ingestion for termination
	ingestCtx, cancel := context.WithCancel(ctx)
	go s.tailAndIngestDeploymentLogs(ingestCtx, stack.ID, terminationDeploymentID, logPath)

	// Update deployment status to in-progress
	_ = s.deploymentRepo.UpdateDeploymentStatus(terminationDeploymentID.String(), entities.DeploymentRunStatusInProgress)

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
		cancel()
		return
	}

	err = s.stackRepo.UpdateStatus(stackId.String(), entities.StackStatusTerminated, "")
	if err != nil {
		logger.Error("failed to update stacks status to terminated",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		_ = s.deploymentRepo.UpdateDeploymentStatus(terminationDeploymentID.String(), entities.DeploymentRunStatusFailed)
		cancel()
		return
	}

	// Update integrations status to terminated
	err = s.integrationRepo.UpdateIntegrationsStatusByStackID(
		stackId.String(),
		entities.DeploymentStatusTerminated,
		[]entities.DeploymentStatus{},
	)
	if err != nil {
		logger.Error("failed to update integrations status to terminated",
			zap.String("stackId", stackId.String()),
			zap.Error(err))
		_ = s.deploymentRepo.UpdateDeploymentStatus(terminationDeploymentID.String(), entities.DeploymentRunStatusFailed)
		cancel()
		return
	}

	_ = s.deploymentRepo.UpdateDeploymentStatus(terminationDeploymentID.String(), entities.DeploymentRunStatusSuccess)
	cancel()

	logger.Info(
		"AWS infrastructure destroyed successfully",
		zap.String("stackId", stackId.String()),
	)
}
