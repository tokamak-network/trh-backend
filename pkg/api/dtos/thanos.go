package dtos

import (
	"errors"
	"regexp"

	"github.com/tokamak-network/trh-backend/internal/consts"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/internal/utils"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	trhSdkAws "github.com/tokamak-network/trh-sdk/pkg/cloud-provider/aws"
	trhSdkTypes "github.com/tokamak-network/trh-sdk/pkg/types"
	trhSdkUtils "github.com/tokamak-network/trh-sdk/pkg/utils"
	"go.uber.org/zap"
)

var chainNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9 ]*$`)

type DeployThanosRequest struct {
	Network                  entities.DeploymentNetwork `json:"network"                  binding:"required" validate:"oneof=Mainnet Testnet LocalDevnet"`
	L1RpcUrl                 string                     `json:"l1RpcUrl"                 binding:"required" validate:"url"`
	L1BeaconUrl              string                     `json:"l1BeaconUrl"              binding:"required" validate:"url"`
	L2BlockTime              int                        `json:"l2BlockTime"              binding:"required" validate:"min=1"` // seconds
	BatchSubmissionFrequency int                        `json:"batchSubmissionFrequency" binding:"required" validate:"min=1"` // seconds
	OutputRootFrequency      int                        `json:"outputRootFrequency"      binding:"required" validate:"min=1"` // seconds
	ChallengePeriod          int                        `json:"challengePeriod"          binding:"required" validate:"min=1"` // seconds
	AdminAccount             string                     `json:"adminAccount"             binding:"required" validate:"eth_address"`
	SequencerAccount         string                     `json:"sequencerAccount"         binding:"required" validate:"eth_address"`
	BatcherAccount           string                     `json:"batcherAccount"           binding:"required" validate:"eth_address"`
	ProposerAccount          string                     `json:"proposerAccount"          binding:"required" validate:"eth_address"`
	AwsAccessKey             string                     `json:"awsAccessKey"             binding:"required"`
	AwsSecretAccessKey       string                     `json:"awsSecretAccessKey"       binding:"required"`
	AwsRegion                string                     `json:"awsRegion"                binding:"required"`
	ChainName                string                     `json:"chainName"                binding:"required"`
	DeploymentPath           string                     `json:"deploymentPath"`
}

func (request *DeployThanosRequest) Validate() error {
	if request.Network == entities.DeploymentNetworkLocalDevnet {
		return errors.New("local devnet is not supported yet")
	}

	// Validate Chain Name
	if !chainNameRegex.MatchString(request.ChainName) {
		logger.Error("invalid chainName", zap.String("chainName", request.ChainName))
		return errors.New(
			"invalid chain name, chain name must contain only letters (a-z, A-Z), numbers (0-9), spaces. Special characters are not allowed",
		)
	}

	// Validate L1 RPC URL
	if !trhSdkUtils.IsValidL1RPC(request.L1RpcUrl) {
		logger.Error("invalid l1RpcUrl", zap.String("l1RpcUrl", request.L1RpcUrl))
		return errors.New("invalid l1RpcUrl")
	}

	// Validate L1 Beacon URL
	if !trhSdkUtils.IsValidBeaconURL(request.L1BeaconUrl) {
		logger.Error("invalid l1BeaconUrl", zap.String("l1BeaconUrl", request.L1BeaconUrl))
		return errors.New("invalid l1BeaconUrl")
	}

	// Validate AWS Access Key
	if !trhSdkUtils.IsValidAWSAccessKey(request.AwsAccessKey) {
		logger.Error("invalid awsAccessKey", zap.String("awsAccessKey", request.AwsAccessKey))
		return errors.New("invalid awsAccessKey")
	}

	// Validate AWS Secret Key
	if !trhSdkUtils.IsValidAWSSecretKey(request.AwsSecretAccessKey) {
		logger.Error(
			"invalid awsSecretKey",
			zap.String("awsSecretAccessKey", request.AwsSecretAccessKey),
		)
		return errors.New("invalid awsSecretKey")
	}

	// Validate AWS Region
	if !trhSdkAws.IsAvailableRegion(
		request.AwsAccessKey,
		request.AwsSecretAccessKey,
		request.AwsRegion,
	) {
		logger.Error("invalid awsRegion", zap.String("awsRegion", request.AwsRegion))
		return errors.New("invalid awsRegion")
	}

	// Validate Chain Config
	chainID, err := utils.GetChainIDFromRPC(request.L1RpcUrl)
	if err != nil {
		logger.Error("invalid rpc", zap.String("chainId", err.Error()))
		return errors.New("invalid rpc")
	}
	chainConfig := trhSdkTypes.ChainConfiguration{
		BatchSubmissionFrequency: uint64(request.BatchSubmissionFrequency),
		OutputRootFrequency:      uint64(request.OutputRootFrequency),
		ChallengePeriod:          uint64(request.ChallengePeriod),
		L2BlockTime:              uint64(request.L2BlockTime),
		L1BlockTime:              consts.L1_BLOCK_TIME,
	}

	err = chainConfig.Validate(chainID)
	if err != nil {
		logger.Error("invalid chainConfig", zap.String("chainConfig", err.Error()))
		return err
	}

	return nil
}

type DeployL1ContractsRequest struct {
	Network                  entities.DeploymentNetwork `json:"network"                  binding:"required" validate:"oneof=Mainnet Testnet LocalDevnet"`
	L1RpcUrl                 string                     `json:"l1RpcUrl"                 binding:"required" validate:"url"`
	L2BlockTime              int                        `json:"l2BlockTime"              binding:"required" validate:"min=1"` // seconds
	BatchSubmissionFrequency int                        `json:"batchSubmissionFrequency" binding:"required" validate:"min=1"` // seconds
	OutputRootFrequency      int                        `json:"outputRootFrequency"      binding:"required" validate:"min=1"` // seconds
	ChallengePeriod          int                        `json:"challengePeriod"          binding:"required" validate:"min=1"` // seconds
	AdminAccount             string                     `json:"adminAccount"             binding:"required" validate:"eth_address"`
	SequencerAccount         string                     `json:"sequencerAccount"         binding:"required" validate:"eth_address"`
	BatcherAccount           string                     `json:"batcherAccount"           binding:"required" validate:"eth_address"`
	ProposerAccount          string                     `json:"proposerAccount"          binding:"required" validate:"eth_address"`
	DeploymentPath           string                     `json:"deploymentPath"           binding:"required"`
	LogPath                  string                     `json:"logPath"                  binding:"required"`
}

type DeployThanosAWSInfraRequest struct {
	ChainName          string `json:"chainName"          binding:"required"`
	Network            string `json:"network"            binding:"required" validate:"oneof=Mainnet Testnet LocalDevnet"`
	L1BeaconUrl        string `json:"l1BeaconUrl"        binding:"required" validate:"url"`
	AwsAccessKey       string `json:"awsAccessKey"       binding:"required"`
	AwsSecretAccessKey string `json:"awsSecretAccessKey" binding:"required"`
	AwsRegion          string `json:"awsRegion"          binding:"required"`
	DeploymentPath     string `json:"deploymentPath"     binding:"required"`
	LogPath            string `json:"logPath"            binding:"required"`
}

type InstallBlockExplorerRequest struct {
	DatabaseUsername string `json:"databaseUsername"     binding:"required"`
	DatabasePassword string `json:"databasePassword"     binding:"required"`
	CoinmarketcapKey string `json:"coinmarketcapKey"     binding:"required"`
	WalletConnectID  string `json:"walletConnectId"     binding:"required"`
}

func (r *InstallBlockExplorerRequest) Validate() error {
	if err := trhSdkUtils.IsValidRDSUsername(r.DatabaseUsername); err {
		logger.Error("invalid database username", zap.String("databaseUsername", r.DatabaseUsername))
		return errors.New("invalid database username")
	}

	if !trhSdkUtils.IsValidRDSPassword(r.DatabasePassword) {
		logger.Error("invalid database password", zap.String("databasePassword", r.DatabasePassword))
		return errors.New("invalid database password")
	}

	if r.CoinmarketcapKey == "" {
		logger.Error("coinmarketcapKey is required")
		return errors.New("coinmarketcapKey is required")
	}
	if r.WalletConnectID == "" {
		logger.Error("walletConnectId is required")
		return errors.New("walletConnectId is required")
	}

	return nil
}

type TerminateThanosRequest struct {
	Network            string `json:"network"            binding:"required" validate:"oneof=Mainnet Testnet LocalDevnet"`
	AwsAccessKey       string `json:"awsAccessKey"       binding:"required"`
	AwsSecretAccessKey string `json:"awsSecretAccessKey" binding:"required"`
	AwsRegion          string `json:"awsRegion"          binding:"required"`
	DeploymentPath     string `json:"deploymentPath"     binding:"required"`
	LogPath            string `json:"logPath"            binding:"required"`
}

type DeployThanosResponse struct {
	Id string `json:"id"`
}

type InstallPluginsRequest struct {
	Plugins []string `json:"plugins"`
}

func (r *InstallPluginsRequest) Validate() error {
	if len(r.Plugins) == 0 {
		return errors.New("no plugins")
	}

	return nil
}
