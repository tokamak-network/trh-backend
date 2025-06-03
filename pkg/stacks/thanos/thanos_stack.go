package thanos

import (
	"context"
	"github.com/tokamak-network/trh-backend/internal/consts"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	trhSDKLogging "github.com/tokamak-network/trh-sdk/pkg/logging"
	thanosStack "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
	thanosTypes "github.com/tokamak-network/trh-sdk/pkg/types"
)

func DeployAWSInfrastructure(req *dtos.DeployThanosAWSInfraRequest) error {
	l := trhSDKLogging.InitLogger(req.LogPath)

	logger.Info("Deploying AWS Infrastructure...")
	return nil

	awsConfig := thanosTypes.AWSConfig{
		AccessKey: req.AwsAccessKey,
		SecretKey: req.AwsSecretAccessKey,
		Region:    req.AwsRegion,
	}

	s, err := thanosStack.NewThanosStack(l, req.Network, true, req.DeploymentPath, &awsConfig)
	if err != nil {
		logger.Errorf("Failed to create thanos stacks: %s", err)
		return err
	}

	deployInfraInput := thanosStack.DeployInfraInput{
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

func DestroyAWSInfrastructure(req *dtos.TerminateThanosRequest) error {
	trhLogger := trhSDKLogging.InitLogger(req.LogPath)

	logger.Info("Destroying AWS Infrastructure...")

	awsConfig := thanosTypes.AWSConfig{
		AccessKey: req.AwsAccessKey,
		SecretKey: req.AwsSecretAccessKey,
		Region:    req.AwsRegion,
	}
	s, err := thanosStack.NewThanosStack(trhLogger, string(req.Network), true, req.DeploymentPath, &awsConfig)
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

func DeployL1Contracts(req *dtos.DeployL1ContractsRequest) error {
	logger.Info("Deploying L1 Contracts...")
	return nil

	trhLogger := trhSDKLogging.InitLogger(req.LogPath)
	s, err := thanosStack.NewThanosStack(trhLogger, string(req.Network), true, req.DeploymentPath, nil)
	if err != nil {
		return err
	}
	chainConfig := thanosTypes.ChainConfiguration{
		BatchSubmissionFrequency: uint64(req.BatchSubmissionFrequency),
		ChallengePeriod:          uint64(req.ChallengePeriod),
		OutputRootFrequency:      uint64(req.OutputRootFrequency),
		L2BlockTime:              uint64(req.L2BlockTime),
		L1BlockTime:              12,
	}
	operators := thanosTypes.Operators{
		AdminPrivateKey:      req.AdminAccount,
		SequencerPrivateKey:  req.SequencerAccount,
		BatcherPrivateKey:    req.BatcherAccount,
		ProposerPrivateKey:   req.ProposerAccount,
		ChallengerPrivateKey: "",
	}

	contractDeploymentInput := thanosStack.DeployContractsInput{
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
