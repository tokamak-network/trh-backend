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

	var registerCandidateParams *dtos.RegisterCandidateRequest
	if config.RegisterCandidate {
		registerCandidateParams = config.RegisterCandidateParams
	}

	existingDeployments, err := s.deploymentRepo.GetDeploymentsByStackID(stackId.String())
	if err != nil {
		return nil, err
	}
	builtContracts := false
	deployedContracts := false
	for _, d := range existingDeployments {
		if d.Step == constants.BuildL1ContractsStep && d.Status == entities.DeploymentRunStatusSuccess {
			builtContracts = true
		}
		if d.Step == constants.DeployL1ContractsStep && d.Status == entities.DeploymentRunStatusSuccess {
			deployedContracts = true
		}
	}

	// Build step: clone + compile contracts only
	if !builtContracts && !deployedContracts {
		buildConfig, err := makeL1ContractsConfig(config, registerCandidateParams, true)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, &entities.DeploymentEntity{
			ID:      uuid.New(),
			StackID: &stackId,
			Step:    constants.BuildL1ContractsStep,
			Status:  entities.DeploymentRunStatusPending,
			LogPath: utils.GetLogPath(stackId, constants.BuildL1ContractsStep),
			Config:  buildConfig,
		})
	}

	// Deploy step: deploy compiled contracts to L1
	if !deployedContracts {
		deployConfig, err := makeL1ContractsConfig(config, registerCandidateParams, false)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, &entities.DeploymentEntity{
			ID:      uuid.New(),
			StackID: &stackId,
			Step:    constants.DeployL1ContractsStep,
			Status:  entities.DeploymentRunStatusPending,
			LogPath: utils.GetLogPath(stackId, constants.DeployL1ContractsStep),
			Config:  deployConfig,
		})
	}

	thanosInfrastructureDeploymentID := uuid.New()
	thanosInfrastructureDeploymentLogPath := utils.GetLogPath(stackId, constants.DeployInfraStep)
	thanosInfrastructureDeploymentConfig, err := json.Marshal(dtos.DeployThanosAWSInfraRequest{
		ChainName:     config.ChainName,
		L1BeaconUrl:   config.L1BeaconUrl,
		BackupConfig:  config.BackupConfig,
		InfraProvider: config.InfraProvider,
	})
	if err != nil {
		return nil, err
	}
	deployments = append(deployments, &entities.DeploymentEntity{
		ID:      thanosInfrastructureDeploymentID,
		StackID: &stackId,
		Step:    constants.DeployInfraStep,
		Status:  entities.DeploymentRunStatusPending,
		LogPath: thanosInfrastructureDeploymentLogPath,
		Config:  thanosInfrastructureDeploymentConfig,
	})

	return deployments, nil
}

// makeL1ContractsConfig builds a serialized DeployL1ContractsRequest from the stack config.
func makeL1ContractsConfig(config *dtos.DeployThanosRequest, registerCandidateParams *dtos.RegisterCandidateRequest, buildOnly bool) ([]byte, error) {
	return json.Marshal(dtos.DeployL1ContractsRequest{
		L1RpcUrl:                 config.L1RpcUrl,
		L2BlockTime:              config.L2BlockTime,
		BatchSubmissionFrequency: config.BatchSubmissionFrequency,
		OutputRootFrequency:      config.OutputRootFrequency,
		ChallengePeriod:          config.ChallengePeriod,
		AdminAccount:             config.AdminAccount,
		SequencerAccount:         config.SequencerAccount,
		BatcherAccount:           config.BatcherAccount,
		ProposerAccount:          config.ProposerAccount,
		ChallengerAccount:        config.ChallengerAccount,
		EnableFaultProof:         config.EnableFaultProof,
		RegisterCandidate:        config.RegisterCandidate,
		RegisterCandidateParams:  registerCandidateParams,
		ReuseDeployment:          config.ReuseDeployment,
		BuildOnly:                buildOnly,
		Preset:                   config.PresetID,
		FeeToken:                 config.FeeToken,
	})
}

// RegisterCandidate moved to pkg/services/thanos/integrations/register_candidate.go and is exposed via
// ThanosStackDeploymentService.RegisterCandidate in service.go
