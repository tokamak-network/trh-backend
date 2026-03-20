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

	if isLocalTarget(config.Network) {
		// Local target: create deploy-local-infra step (kind + Helm, no AWS)
		localInfraDeploymentID := uuid.New()
		localInfraDeploymentLogPath := utils.GetLogPath(stackId, constants.DeployLocalInfraStep)
		localInfraDeploymentConfig, err := json.Marshal(map[string]string{
			"kubeconfigPath": config.KubeconfigPath,
			"chainName":      config.ChainName,
			"l1RpcUrl":       config.L1RpcUrl,
			"l1BeaconUrl":    config.L1BeaconUrl,
		})
		if err != nil {
			return nil, err
		}
		localInfraDeployment := &entities.DeploymentEntity{
			ID:      localInfraDeploymentID,
			StackID: &stackId,
			Step:    constants.DeployLocalInfraStep,
			Status:  entities.DeploymentRunStatusPending,
			LogPath: localInfraDeploymentLogPath,
			Config:  localInfraDeploymentConfig,
		}
		deployments = append(deployments, localInfraDeployment)
	} else {
		// Mainnet/Testnet: create deploy-aws-infra step
		thanosInfrastructureDeploymentID := uuid.New()
		thanosInfrastructureDeploymentLogPath := utils.GetLogPath(stackId, constants.DeployInfraStep)
		thanosInfrastructureDeploymentConfig, err := json.Marshal(dtos.DeployThanosAWSInfraRequest{
			ChainName:    config.ChainName,
			L1BeaconUrl:  config.L1BeaconUrl,
			BackupConfig: config.BackupConfig,
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
	}

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
		RegisterCandidate:        config.RegisterCandidate,
		RegisterCandidateParams:  registerCandidateParams,
		BuildOnly:                buildOnly,
		Preset:                   config.PresetID,
		FeeToken:                 config.FeeToken,
	})
}

// RegisterCandidate moved to pkg/services/thanos/integrations/register_candidate.go and is exposed via
// ThanosStackDeploymentService.RegisterCandidate in service.go
