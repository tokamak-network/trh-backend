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

func getThanosStackDeployments(
	stackId uuid.UUID,
	config *dtos.DeployThanosRequest,
) ([]*entities.DeploymentEntity, error) {
	deployments := make([]*entities.DeploymentEntity, 0)
	l1ContractDeploymentID := uuid.New()
	l1ContractDeploymentLogPath := utils.GetLogPath(stackId, "deploy-l1-contracts")

	var registerCandidateParams *dtos.RegisterCandidateRequest
	if config.RegisterCandidate {
		registerCandidateParams = config.RegisterCandidateParams
	}

	l1ContractDeploymentConfig, err := json.Marshal(dtos.DeployL1ContractsRequest{
		L1RpcUrl:                 config.L1RpcUrl,
		L2BlockTime:              config.L2BlockTime,
		BatchSubmissionFrequency: config.BatchSubmissionFrequency,
		OutputRootFrequency:      config.OutputRootFrequency,
		ChallengePeriod:          config.ChallengePeriod,
		AdminAccount:             config.AdminAccount,
		SequencerAccount:         config.SequencerAccount,
		BatcherAccount:           config.BatcherAccount,
		ProposerAccount:          config.ProposerAccount,
		RegisterCandidate:        config.RegisterCandidate,
		RegisterCandidateParams:  registerCandidateParams,
	})
	if err != nil {
		return nil, err
	}
	l1ContractDeployment := &entities.DeploymentEntity{
		ID:      l1ContractDeploymentID,
		StackID: &stackId,
		Step:    1,
		Status:  entities.DeploymentStatusPending,
		LogPath: l1ContractDeploymentLogPath,
		Config:  l1ContractDeploymentConfig,
	}
	deployments = append(deployments, l1ContractDeployment)

	thanosInfrastructureDeploymentID := uuid.New()
	thanosInfrastructureDeploymentLogPath := utils.GetLogPath(
		stackId,
		"deploy-thanos-aws-infra",
	)
	thanosInfrastructureDeploymentConfig, err := json.Marshal(dtos.DeployThanosAWSInfraRequest{
		ChainName:   config.ChainName,
		L1BeaconUrl: config.L1BeaconUrl,
	})
	if err != nil {
		return nil, err
	}
	thanosInfrastructureDeployment := &entities.DeploymentEntity{
		ID:      thanosInfrastructureDeploymentID,
		StackID: &stackId,
		Step:    2,
		Status:  entities.DeploymentStatusPending,
		LogPath: thanosInfrastructureDeploymentLogPath,
		Config:  thanosInfrastructureDeploymentConfig,
	}
	deployments = append(deployments, thanosInfrastructureDeployment)

	return deployments, nil
}

func (s *ThanosStackDeploymentService) RegisterCandidate(ctx context.Context, stackId uuid.UUID, req dtos.RegisterCandidateRequest) (*entities.Response, error) {
	stack, err := s.stackRepo.GetStackByID(stackId.String())
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
	integrations, err := s.integrationRepo.GetActiveIntegrations(stackId.String(), enum.IntegrationTypeRegisterCandidate.String())
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

	s.taskManager.AddTask(taskId, func(ctx context.Context) {
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
			LogPath: registerCandidateLogPath,
		}
		err = s.integrationRepo.CreateIntegration(integration)
		if err != nil {
			logger.Error("failed to create integration", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()), zap.Error(err))
			return
		}
		err = thanos.VerifyRegisterCandidates(ctx, sdkClient, &req)
		if err != nil {
			logger.Error("failed to register candidate", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()), zap.Error(err), zap.String("stackId", stackId.String()))
			err = s.integrationRepo.UpdateIntegrationStatusWithReason(integrationId.String(), entities.DeploymentStatusFailed, err.Error())
			if err != nil {
				logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()), zap.Error(err), zap.String("integrationId", integrationId.String()))
			}
			return
		}
		err = s.integrationRepo.UpdateIntegrationStatus(integrationId.String(), entities.DeploymentStatusCompleted)
		if err != nil {
			logger.Error("failed to update integration status", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()), zap.Error(err), zap.String("integrationId", integrationId.String()))
		}

		registerCandidateInfo, err := thanos.GetRegisterCandidatesInfo(ctx, sdkClient, &req)
		if err != nil {
			logger.Error("failed to get register candidate info", zap.Error(err))
			return
		}

		bytes, err := json.Marshal(registerCandidateInfo)
		if err != nil {
			logger.Error("failed to marshal register candidate info", zap.Error(err))
			return
		}

		err = s.integrationRepo.UpdateMetadataAfterInstalled(
			integrationId.String(),
			bytes,
		)

		if err != nil {
			logger.Error("failed to update register candidate integration metadata", zap.String("plugin", enum.IntegrationTypeRegisterCandidate.String()), zap.Error(err))
			return
		}

		logger.Info("Register candidate successfully", zap.String("stackId", stackId.String()))
	})

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Candidate registered successfully",
		Data:    nil,
	}, nil
}
