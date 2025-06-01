package trh_sdk

import (
	"context"
	"github.com/tokamak-network/trh-backend/internal/consts"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/interfaces/api/dtos"

	trh_sdk_logging "github.com/tokamak-network/trh-sdk/pkg/logging"
	trh_sdk_thanos "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
	trh_sdk_types "github.com/tokamak-network/trh-sdk/pkg/types"
)

// TODO: This is a mock implementation of the ThanosStack interface.
// We should use the actual implementation of the ThanosStack interface.
type ThanosStack interface {
	DeployAWSInfrastructure(req *dtos.DeployThanosAWSInfraRequest) error
	DestroyAWSInfrastructure(req *dtos.TerminateThanosRequest) error
	DeployL1Contracts(req *dtos.DeployL1ContractsRequest) error
}

type ThanosStackImpl struct {
}

func NewThanosStack() ThanosStack {
	return &ThanosStackImpl{}
}

func (t *ThanosStackImpl) DeployAWSInfrastructure(req *dtos.DeployThanosAWSInfraRequest) error {
	trh_logger := trh_sdk_logging.InitLogger(req.LogPath)

	logger.Info("Deploying AWS Infrastructure...")

	awsConfig := trh_sdk_types.AWSConfig{
		AccessKey: req.AwsAccessKey,
		SecretKey: req.AwsSecretAccessKey,
		Region:    req.AwsRegion,
	}

	s, err := trh_sdk_thanos.NewThanosStack(trh_logger, string(req.Network), true, req.DeploymentPath, &awsConfig)
	if err != nil {
		return err
	}

	deployInfraInput := trh_sdk_thanos.DeployInfraInput{
		ChainName:   req.ChainName,
		L1BeaconURL: req.L1BeaconUrl,
	}

	err = s.Deploy(context.Background(), consts.AWS, &deployInfraInput)
	if err != nil {
		return err
	}

	logger.Info("AWS Infrastructure deployed successfully")

	return nil
}

func (t *ThanosStackImpl) DestroyAWSInfrastructure(req *dtos.TerminateThanosRequest) error {
	trh_logger := trh_sdk_logging.InitLogger(req.LogPath)

	logger.Info("Destroying AWS Infrastructure...")

	awsConfig := trh_sdk_types.AWSConfig{
		AccessKey: req.AwsAccessKey,
		SecretKey: req.AwsSecretAccessKey,
		Region:    req.AwsRegion,
	}
	s, err := trh_sdk_thanos.NewThanosStack(trh_logger, string(req.Network), true, req.DeploymentPath, &awsConfig)
	if err != nil {
		return err
	}

	err = s.Destroy(context.Background())
	if err != nil {
		return err
	}

	logger.Info("AWS Infrastructure destroyed successfully")

	return nil
}

func (t *ThanosStackImpl) DeployL1Contracts(req *dtos.DeployL1ContractsRequest) error {
	logger.Info("Deploying L1 Contracts...")
	trh_logger := trh_sdk_logging.InitLogger(req.LogPath)
	s, err := trh_sdk_thanos.NewThanosStack(trh_logger, string(req.Network), true, req.DeploymentPath, nil)
	if err != nil {
		return err
	}
	chainConfig := trh_sdk_types.ChainConfiguration{
		BatchSubmissionFrequency: uint64(req.BatchSubmissionFrequency),
		ChallengePeriod:          uint64(req.ChallengePeriod),
		OutputRootFrequency:      uint64(req.OutputRootFrequency),
		L2BlockTime:              uint64(req.L2BlockTime),
		L1BlockTime:              12,
	}
	operators := trh_sdk_types.Operators{
		AdminPrivateKey:      req.AdminAccount,
		SequencerPrivateKey:  req.SequencerAccount,
		BatcherPrivateKey:    req.BatcherAccount,
		ProposerPrivateKey:   req.ProposerAccount,
		ChallengerPrivateKey: "",
	}

	contractDeploymentInput := trh_sdk_thanos.DeployContractsInput{
		L1RPCurl:           req.L1RpcUrl,
		ChainConfiguration: &chainConfig,
		Operators:          &operators,
	}
	err = s.DeployContracts(context.Background(), &contractDeploymentInput)
	if err != nil {
		return err
	}

	logger.Info("L1 Contracts deployed successfully")
	return nil
}
