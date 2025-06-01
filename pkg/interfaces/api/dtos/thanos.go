package dtos

import "github.com/tokamak-network/trh-backend/pkg/domain/entities"

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
	ChainName          string `json:"chainName" binding:"required"`
	Network            string `json:"network" binding:"required" validate:"oneof=Mainnet Testnet LocalDevnet"`
	L1BeaconUrl        string `json:"l1BeaconUrl" binding:"required" validate:"url"`
	AwsAccessKey       string `json:"awsAccessKey" binding:"required"`
	AwsSecretAccessKey string `json:"awsSecretAccessKey" binding:"required"`
	AwsRegion          string `json:"awsRegion" binding:"required"`
	DeploymentPath     string `json:"deploymentPath" binding:"required"`
	LogPath            string `json:"logPath" binding:"required"`
}

type TerminateThanosRequest struct {
	Network            string `json:"network" binding:"required" validate:"oneof=Mainnet Testnet LocalDevnet"`
	AwsAccessKey       string `json:"awsAccessKey" binding:"required"`
	AwsSecretAccessKey string `json:"awsSecretAccessKey" binding:"required"`
	AwsRegion          string `json:"awsRegion" binding:"required"`
	DeploymentPath     string `json:"deploymentPath" binding:"required"`
	LogPath            string `json:"logPath" binding:"required"`
}

type DeployThanosResponse struct {
	Id string `json:"id"`
}
