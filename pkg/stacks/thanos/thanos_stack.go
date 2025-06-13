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

func NewThanosSDKClient(
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

func DeployAWSInfrastructure(ctx context.Context, sdkClient *thanosStack.ThanosStack, req *dtos.DeployThanosAWSInfraRequest) error {
	logger.Info("Deploying AWS Infrastructure...")

	deployInfraInput := thanosStack.DeployInfraInput{
		ChainName:   req.ChainName,
		L1BeaconURL: req.L1BeaconUrl,
	}

	err := sdkClient.Deploy(ctx, consts.AWS, &deployInfraInput)
	if err != nil {
		return err
	}

	logger.Info("AWS Infrastructure deployed successfully")

	return nil
}

func DestroyAWSInfrastructure(ctx context.Context, sdkClient *thanosStack.ThanosStack) error {
	logger.Info("Destroying AWS Infrastructure...")

	err := sdkClient.Destroy(ctx)
	if err != nil {
		return err
	}

	logger.Info("AWS Infrastructure destroyed successfully")

	return nil
}

func DeployL1Contracts(ctx context.Context, sdkClient *thanosStack.ThanosStack, req *dtos.DeployL1ContractsRequest) error {
	logger.Info("Deploying L1 Contracts...")

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
		ChallengerPrivateKey: "", // TODO: enable challenger in the future when we support fp
	}

	contractDeploymentInput := thanosStack.DeployContractsInput{
		L1RPCurl:           req.L1RpcUrl,
		ChainConfiguration: &chainConfig,
		Operators:          &operators,
	}
	err := sdkClient.DeployContracts(ctx, &contractDeploymentInput)
	if err != nil {
		return err
	}

	logger.Info("L1 Contracts deployed successfully")
	return nil
}

func ShowChainInformation(
	ctx context.Context,
	sdkClient *thanosStack.ThanosStack,
) (*thanosTypes.ChainInformation, error) {
	return sdkClient.ShowInformation(ctx)
}

func InstallBridge(
	ctx context.Context,
	sdkClient *thanosStack.ThanosStack,
) (string, error) {
	return sdkClient.InstallBridge(ctx)
}

func UninstallBridge(
	ctx context.Context,
	sdkClient *thanosStack.ThanosStack,
) error {
	return sdkClient.UninstallBridge(ctx)
}

func InstallBlockExplorer(
	ctx context.Context,
	s *thanosStack.ThanosStack,
	req *dtos.InstallBlockExplorerRequest,
) (string, error) {
	return s.InstallBlockExplorer(ctx, &thanosStack.InstallBlockExplorerInput{
		DatabaseUsername:       req.DatabaseUsername,
		DatabasePassword:       req.DatabasePassword,
		CoinmarketcapKey:       req.CoinmarketcapKey,
		WalletConnectProjectID: req.WalletConnectID,
	})
}

func UninstallBlockExplorer(
	ctx context.Context,
	s *thanosStack.ThanosStack,
) error {
	return s.UninstallBlockExplorer(ctx)
}

func UpdateNetwork(
	ctx context.Context,
	s *thanosStack.ThanosStack,
	req *dtos.UpdateNetworkRequest,
) error {
	return s.UpdateNetwork(ctx, &thanosStack.UpdateNetworkInput{
		L1RPC:       req.L1RpcUrl,
		L1BeaconURL: req.L1BeaconUrl,
	})
}
