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

func newThanosSDKClient(
	logPath string,
	network string,
	deploymentPath string,
	awsAccessKey string,
	awsSecretAccessKey string,
	awsRegion string,

) (*thanosStack.ThanosStack, error) {
	l := trhSDKLogging.InitLogger(logPath)

	logger.Info("Deploying AWS Infrastructure...")

	var awsConfig *thanosTypes.AWSConfig

	if awsAccessKey != "" && awsSecretAccessKey != "" && awsRegion != "" {
		awsConfig = &thanosTypes.AWSConfig{
			AccessKey: awsAccessKey,
			SecretKey: awsSecretAccessKey,
			Region:    awsRegion,
		}
	}

	s, err := thanosStack.NewThanosStack(l, network, false, deploymentPath, awsConfig)
	if err != nil {
		logger.Errorf("Failed to create thanos stacks: %s", err)
		return nil, err
	}

	return s, nil
}

func DeployAWSInfrastructure(req *dtos.DeployThanosAWSInfraRequest) error {
	logger.Info("Deploying AWS Infrastructure...")

	s, err := newThanosSDKClient(
		req.LogPath,
		req.Network,
		req.DeploymentPath,
		req.AwsAccessKey,
		req.AwsSecretAccessKey,
		req.AwsRegion,
	)
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
	logger.Info("Destroying AWS Infrastructure...")

	s, err := newThanosSDKClient(
		req.LogPath,
		req.Network,
		req.DeploymentPath,
		req.AwsAccessKey,
		req.AwsSecretAccessKey,
		req.AwsRegion,
	)
	if err != nil {
		logger.Errorf("Failed to create thanos stacks: %s", err)
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

	s, err := newThanosSDKClient(
		req.LogPath,
		string(req.Network),
		req.DeploymentPath,
		"",
		"",
		"",
	)
	if err != nil {
		logger.Errorf("Failed to create thanos stacks: %s", err)
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

func ShowChainInformation(
	ctx context.Context,
	logPath string,
	network string,
	deploymentPath string,
	awsAccessKey string,
	awsSecretAccessKey string,
	awsRegion string,
) (*thanosTypes.ChainInformation, error) {
	s, err := newThanosSDKClient(logPath, network, deploymentPath, awsAccessKey, awsSecretAccessKey, awsRegion)
	if err != nil {
		return nil, err
	}

	return s.ShowInformation(ctx)
}
