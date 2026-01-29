package dtos

import (
	"errors"
	"strings"

	trhSdkUtils "github.com/tokamak-network/trh-sdk/pkg/utils"
)

// DRBDatabaseConfig represents the database configuration for DRB
type DRBDatabaseConfig struct {
	Type     string `json:"type" binding:"required"`     // only "rds" is supported
	Username string `json:"username" binding:"required"` // RDS username
	Password string `json:"password" binding:"required"` // RDS password
}

// DRBAWSConfig represents the AWS configuration for DRB
type DRBAWSConfig struct {
	AccessKeyId     string `json:"accessKeyId" binding:"required"`
	SecretAccessKey string `json:"secretAccessKey" binding:"required"`
	Region          string `json:"region" binding:"required"`
}

// InstallDRBRequest represents the request body for installing DRB
// Note: Only leader node deployment is currently supported
type InstallDRBRequest struct {
	// Network Configuration
	UseCurrentChain bool   `json:"useCurrentChain"`   // if true, use the deployed chain RPC and chain ID
	RPC             string `json:"rpc,omitempty"`     // custom RPC URL (required if useCurrentChain is false)
	ChainID         uint64 `json:"chainId,omitempty"` // custom chain ID (required if useCurrentChain is false)

	// Deployer Configuration
	PrivateKey string `json:"privateKey" binding:"required"` // deployer private key

	// AWS Configuration
	AWSConfig *DRBAWSConfig `json:"awsConfig" binding:"required"`

	// Database Configuration
	DatabaseConfig *DRBDatabaseConfig `json:"databaseConfig" binding:"required"`
}

// Validate validates the InstallDRBRequest
// Note: Required field checks are handled by binding tags. This validates business logic only.
func (r *InstallDRBRequest) Validate() error {
	// Custom network requires RPC and ChainID
	if !r.UseCurrentChain {
		if r.RPC == "" {
			return errors.New("rpc is required when useCurrentChain is false")
		}
		if !trhSdkUtils.IsValidL1RPC(r.RPC) {
			return errors.New("invalid RPC URL")
		}
		if r.ChainID == 0 {
			return errors.New("chainId is required when useCurrentChain is false")
		}
	}

	// Private key format (64 hex chars, with optional 0x prefix)
	if len(strings.TrimPrefix(r.PrivateKey, "0x")) != 64 {
		return errors.New("invalid private key format")
	}

	// Database type must be "rds"
	if r.DatabaseConfig.Type != "rds" {
		return errors.New("databaseConfig.type must be 'rds'")
	}

	// Domain-specific format validations
	if !trhSdkUtils.IsValidRDSUsername(r.DatabaseConfig.Username) {
		return errors.New("invalid RDS username")
	}
	if !trhSdkUtils.IsValidRDSPassword(r.DatabaseConfig.Password) {
		return errors.New("invalid RDS password")
	}

	return nil
}

// DRBContractInfo represents the deployed DRB contract information
type DRBContractInfo struct {
	ContractAddress          string `json:"contractAddress"`
	ContractName             string `json:"contractName"`                       // "CommitReveal2" or "CommitReveal2L2"
	ChainID                  uint64 `json:"chainId"`
	ConsumerExampleV2Address string `json:"consumerExampleV2Address,omitempty"` // optional consumer contract
}

// DRBApplicationInfo represents the deployed DRB application information
type DRBApplicationInfo struct {
	LeaderNodeURL string `json:"leaderNodeUrl"`
}

// DRBLeaderInfo represents the full leader node connection information
type DRBLeaderInfo struct {
	LeaderURL                string `json:"leaderUrl"`
	LeaderIP                 string `json:"leaderIp"`
	LeaderPort               int    `json:"leaderPort"`
	LeaderPeerID             string `json:"leaderPeerId"`
	LeaderEOA                string `json:"leaderEoa"`
	CommitReveal2L2Address   string `json:"commitReveal2L2Address"`
	ConsumerExampleV2Address string `json:"consumerExampleV2Address,omitempty"`
	ChainID                  uint64 `json:"chainId"`
	RPCURL                   string `json:"rpcUrl"`
	DeploymentTimestamp      string `json:"deploymentTimestamp"`
	ClusterName              string `json:"clusterName"`
	Namespace                string `json:"namespace"`
}

// DRBDeploymentInfo represents the full DRB deployment information
type DRBDeploymentInfo struct {
	Contract     *DRBContractInfo    `json:"contract"`
	Application  *DRBApplicationInfo `json:"application"`
	LeaderInfo   *DRBLeaderInfo      `json:"leaderInfo,omitempty"`
	DatabaseType string              `json:"databaseType"`
}
