package services

import (
	"encoding/json"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	"github.com/tokamak-network/trh-backend/pkg/interfaces/api/dtos"

	"github.com/google/uuid"
)

type ThanosDomainService struct {
}

func NewThanosDomainService() *ThanosDomainService {
	return &ThanosDomainService{}
}

func (s *ThanosDomainService) GetThanosStackDeployments(stackId uuid.UUID, config *dtos.DeployThanosRequest, deploymentPath string) ([]entities.DeploymentEntity, error) {
	deployments := []entities.DeploymentEntity{}
	l1ContractDeploymentID := uuid.New()
	l1ContractDeploymentLogPath := utils.GetDeploymentLogPath(stackId, l1ContractDeploymentID)
	l1ContractDeploymentConfig, err := json.Marshal(dtos.DeployL1ContractsRequest{
		Network:                  config.Network,
		L1RpcUrl:                 config.L1RpcUrl,
		L2BlockTime:              config.L2BlockTime,
		BatchSubmissionFrequency: config.BatchSubmissionFrequency,
		OutputRootFrequency:      config.OutputRootFrequency,
		ChallengePeriod:          config.ChallengePeriod,
		AdminAccount:             config.AdminAccount,
		SequencerAccount:         config.SequencerAccount,
		BatcherAccount:           config.BatcherAccount,
		ProposerAccount:          config.ProposerAccount,
		DeploymentPath:           deploymentPath,
		LogPath:                  l1ContractDeploymentLogPath,
	})
	if err != nil {
		return nil, err
	}
	l1ContractDeployment := entities.DeploymentEntity{
		ID:             l1ContractDeploymentID,
		StackID:        &stackId,
		IntegrationID:  nil,
		Step:           1,
		Name:           "thanos-l1-contract-deployment",
		Status:         entities.DeploymentStatusPending,
		LogPath:        l1ContractDeploymentLogPath,
		Config:         l1ContractDeploymentConfig,
		DeploymentPath: deploymentPath,
	}
	deployments = append(deployments, l1ContractDeployment)

	thanosInfrastructureDeploymentID := uuid.New()
	thanosInfrastructureDeploymentLogPath := utils.GetDeploymentLogPath(stackId, thanosInfrastructureDeploymentID)
	thanosInfrastructureDeploymentConfig, err := json.Marshal(dtos.DeployThanosAWSInfraRequest{
		ChainName:          config.ChainName,
		Network:            string(config.Network),
		L1BeaconUrl:        config.L1BeaconUrl,
		AwsAccessKey:       config.AwsAccessKey,
		AwsSecretAccessKey: config.AwsSecretAccessKey,
		AwsRegion:          config.AwsRegion,
		DeploymentPath:     deploymentPath,
		LogPath:            thanosInfrastructureDeploymentLogPath,
	})
	if err != nil {
		return nil, err
	}
	thanosInfrastructureDeployment := entities.DeploymentEntity{
		ID:             thanosInfrastructureDeploymentID,
		StackID:        &stackId,
		IntegrationID:  nil,
		Step:           2,
		Name:           "thanos-infrastructure-deployment",
		Status:         entities.DeploymentStatusPending,
		LogPath:        thanosInfrastructureDeploymentLogPath,
		Config:         thanosInfrastructureDeploymentConfig,
		DeploymentPath: deploymentPath,
	}
	deployments = append(deployments, thanosInfrastructureDeployment)

	return deployments, nil
}
