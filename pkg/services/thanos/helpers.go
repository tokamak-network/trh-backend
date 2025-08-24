package thanos

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/constants"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

func (s *ThanosStackDeploymentService) getThanosStackDeployments(
	stackId uuid.UUID,
	config *dtos.DeployThanosRequest,
) ([]*entities.DeploymentEntity, error) {
	deployments := make([]*entities.DeploymentEntity, 0)
	l1ContractDeploymentID := uuid.New()
	l1ContractDeploymentLogPath := utils.GetLogPath(stackId, constants.DeployL1ContractsStep)

	var registerCandidateParams *dtos.RegisterCandidateRequest
	if config.RegisterCandidate {
		registerCandidateParams = config.RegisterCandidateParams
	}

	deployContracts, err := s.deploymentRepo.GetDeploymentsByStackID(stackId.String())
	if err != nil {
		return nil, err
	}
	deployedContracts := false
	for _, d := range deployContracts {
		if d.Step == constants.DeployL1ContractsStep && d.Status == entities.DeploymentRunStatusSuccess {
			deployedContracts = true
		}
	}
	if !deployedContracts {

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
			Step:    constants.DeployL1ContractsStep,
			Status:  entities.DeploymentRunStatusPending,
			LogPath: l1ContractDeploymentLogPath,
			Config:  l1ContractDeploymentConfig,
		}
		deployments = append(deployments, l1ContractDeployment)
	}

	thanosInfrastructureDeploymentID := uuid.New()
	thanosInfrastructureDeploymentLogPath := utils.GetLogPath(
		stackId,
		constants.DeployInfraStep,
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
		Step:    constants.DeployInfraStep,
		Status:  entities.DeploymentRunStatusPending,
		LogPath: thanosInfrastructureDeploymentLogPath,
		Config:  thanosInfrastructureDeploymentConfig,
	}
	deployments = append(deployments, thanosInfrastructureDeployment)

	return deployments, nil
}

// RegisterCandidate moved to pkg/services/thanos/integrations/register_candidate.go and is exposed via
// ThanosStackDeploymentService.RegisterCandidate in service.go
