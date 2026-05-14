package thanos

import (
	"encoding/json"
	"strings"

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
	// LocalDevnet deploys L1+L2 together in the deploy-local-infra step via SDK.Deploy().
	// Testnet/Mainnet with local infra still requires a separate L1 contract deployment.
	if !deployedContracts && config.Network != entities.DeploymentNetworkLocalDevnet {

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
			ChallengerAccount:        config.ChallengerAccount,
			EnableFaultProof:         config.EnableFaultProof,
			RegisterCandidate:        config.RegisterCandidate,
			RegisterCandidateParams:  registerCandidateParams,
			Preset:                   config.PresetID,
			FeeToken:                 config.FeeToken,
			SeedPhrase:               config.SeedPhrase,
			ReuseDeployment:          config.ReuseDeployment,
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
	infraStepName := constants.GetDeployInfraStepName(config.InfraProvider)
	thanosInfrastructureDeploymentLogPath := utils.GetLogPath(
		stackId,
		infraStepName,
	)
	thanosInfrastructureDeploymentConfig, err := json.Marshal(dtos.DeployThanosAWSInfraRequest{
		ChainName:     config.ChainName,
		L1BeaconUrl:   config.L1BeaconUrl,
		BackupConfig:  config.BackupConfig,
		InfraProvider: config.InfraProvider,
		SeedPhrase:    config.SeedPhrase,
	})
	if err != nil {
		return nil, err
	}
	thanosInfrastructureDeployment := &entities.DeploymentEntity{
		ID:      thanosInfrastructureDeploymentID,
		StackID: &stackId,
		Step:    infraStepName,
		Status:  entities.DeploymentRunStatusPending,
		LogPath: thanosInfrastructureDeploymentLogPath,
		Config:  thanosInfrastructureDeploymentConfig,
	}
	deployments = append(deployments, thanosInfrastructureDeployment)

	return deployments, nil
}

// toSDKNetwork maps a DeploymentNetwork entity value to the SDK network constant string.
// strings.ToLower alone is insufficient: "LocalDevnet" → "localdevnet" but SDK expects "local_devnet".
func toSDKNetwork(n entities.DeploymentNetwork) string {
	switch n {
	case entities.DeploymentNetworkLocalDevnet:
		return "local_devnet"
	case entities.DeploymentNetworkTestnet:
		return "testnet"
	case entities.DeploymentNetworkMainnet:
		return "mainnet"
	default:
		return strings.ToLower(string(n))
	}
}

// RegisterCandidate moved to pkg/services/thanos/integrations/register_candidate.go and is exposed via
// ThanosStackDeploymentService.RegisterCandidate in service.go
