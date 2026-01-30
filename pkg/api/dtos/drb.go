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
type InstallDRBRequest struct {
	NodeType string `json:"nodeType" binding:"required"`

	// Network Configuration
	UseCurrentChain bool   `json:"useCurrentChain"`
	RPC             string `json:"rpc,omitempty"`     // comma separated for fallback support
	ChainID         uint64 `json:"chainId,omitempty"`

	// Deployer Configuration (for leader node)
	PrivateKey string `json:"privateKey,omitempty"` // deployer private key

	// Leader Connection (for regular nodes)
	LeaderIP       string `json:"leaderIp,omitempty"`
	LeaderPort     int    `json:"leaderPort,omitempty"`
	LeaderPeerID   string `json:"leaderPeerId,omitempty"`
	LeaderEOA      string `json:"leaderEoa,omitempty"`
	ContractAddress string `json:"contractAddress,omitempty"`

	// Regular Node Configuration
	NodePort      int    `json:"nodePort,omitempty"`
	EOAPrivateKey string `json:"eoaPrivateKey,omitempty"` // regular node's private key

	// AWS Configuration
	AWSConfig *DRBAWSConfig `json:"awsConfig" binding:"required"`

	// EC2 Configuration for regular nodes)
	EC2Config *DRBEC2Config `json:"ec2Config,omitempty"`

	// Database Configuration
	DatabaseConfig *DRBDatabaseConfig `json:"databaseConfig" binding:"required"`
}

// this represents the EC2 configuration for regular nodes
type DRBEC2Config struct {
	InstanceType string `json:"instanceType,omitempty"` // e.g. "t3.medium"
	KeyPairName  string `json:"keyPairName"`            // SSH key pair name
	SubnetID     string `json:"subnetId,omitempty"`     // optional subnet ID
	InstanceName string `json:"instanceName,omitempty"` // optional instance name
}

func validateRPCUrls(rpcUrls string) error {
	urls := strings.Split(rpcUrls, ",")
	validCount := 0
	for _, url := range urls {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		if !trhSdkUtils.IsValidL1RPC(url) {
			return errors.New("invalid RPC URL: " + url)
		}
		validCount++
	}
	if validCount == 0 {
		return errors.New("at least one valid RPC URL is required")
	}
	return nil
}

func (r *InstallDRBRequest) Validate() error {
	if r.NodeType != "leader" && r.NodeType != "regular" {
		return errors.New("nodeType must be 'leader' or 'regular'")
	}

	if r.NodeType == "leader" {
		if !r.UseCurrentChain {
			if r.RPC == "" {
				return errors.New("rpc is required when useCurrentChain is false")
			}
			if err := validateRPCUrls(r.RPC); err != nil {
				return err
			}
			if r.ChainID == 0 {
				return errors.New("chainId is required when useCurrentChain is false")
			}
		}

		if len(strings.TrimPrefix(r.PrivateKey, "0x")) != 64 {
			return errors.New("invalid private key format")
		}
	} else {
		if r.RPC == "" {
			return errors.New("rpc is required for regular nodes")
		}
		if err := validateRPCUrls(r.RPC); err != nil {
			return err
		}
		if r.ChainID == 0 {
			return errors.New("chainId is required for regular nodes")
		}
		if strings.TrimSpace(r.LeaderIP) == "" {
			return errors.New("leaderIp is required for regular nodes")
		}
		if r.LeaderPort == 0 {
			return errors.New("leaderPort is required for regular nodes")
		}
		if strings.TrimSpace(r.LeaderPeerID) == "" {
			return errors.New("leaderPeerId is required for regular nodes")
		}
		if strings.TrimSpace(r.LeaderEOA) == "" {
			return errors.New("leaderEoa is required for regular nodes")
		}
		if strings.TrimSpace(r.ContractAddress) == "" {
			return errors.New("contractAddress is required for regular nodes")
		}
		// if r.NodePort == 0 {
		// 	r.NodePort = 61281
		// }
		if r.NodePort == 0 {
			r.NodePort = 61280 // match SDK default
		}
		if r.NodePort < 1 || r.NodePort > 65535 {
			return errors.New("nodePort must be between 1 and 65535")
		}
		if r.LeaderPort < 1 || r.LeaderPort > 65535 {
			return errors.New("leaderPort must be between 1 and 65535")
		}
		if len(strings.TrimPrefix(r.EOAPrivateKey, "0x")) != 64 {
			return errors.New("invalid eoaPrivateKey format")
		}
		if r.EC2Config == nil {
			return errors.New("ec2Config is required for regular nodes")
		}
		if strings.TrimSpace(r.EC2Config.KeyPairName) == "" {
			return errors.New("ec2Config.keyPairName is required for regular nodes")
		}
	}

	// DB validation
	if r.DatabaseConfig.Type != "rds" && r.DatabaseConfig.Type != "local" {
		return errors.New("databaseConfig.type must be 'rds' or 'local'")
	}

	if r.DatabaseConfig.Type == "rds" {
		if !trhSdkUtils.IsValidRDSUsername(r.DatabaseConfig.Username) {
			return errors.New("invalid RDS username")
		}
		if !trhSdkUtils.IsValidRDSPassword(r.DatabaseConfig.Password) {
			return errors.New("invalid RDS password")
		}
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

// this will represents the deployed regular node information
type DRBRegularNodeInfo struct {
	NodeURL       string `json:"nodeUrl"`
	NodeIP        string `json:"nodeIp"`
	NodePort      int    `json:"nodePort"`
	NodePeerID    string `json:"nodePeerId,omitempty"`
	NodeEOA       string `json:"nodeEoa"`
	InstanceID    string `json:"instanceId,omitempty"`    // ec2 instance id
	InstanceType  string `json:"instanceType,omitempty"`  // ec2 instance type
	Region        string `json:"region"`
	ChainID       uint64 `json:"chainId"`
	RPCURL        string `json:"rpcUrl"`
	LeaderIP      string `json:"leaderIp"`
	LeaderPort    int    `json:"leaderPort"`
	LeaderPeerID  string `json:"leaderPeerId"`
	LeaderEOA     string `json:"leaderEoa"`
	ContractAddress string `json:"contractAddress"`
	DeploymentTimestamp string `json:"deploymentTimestamp"`
}

// DRBDeploymentInfo represents the full DRB deployment information
type DRBDeploymentInfo struct {
	NodeType        string              `json:"nodeType"`                  // "leader" or "regular"
	Contract        *DRBContractInfo    `json:"contract,omitempty"`        // only for leader nodes
	Application     *DRBApplicationInfo `json:"application,omitempty"`     // only for leader nodes
	LeaderInfo      *DRBLeaderInfo      `json:"leaderInfo,omitempty"`      // only for leader nodes
	RegularNodeInfo *DRBRegularNodeInfo `json:"regularNodeInfo,omitempty"` // only for regular nodes
	DatabaseType    string              `json:"databaseType"`
}

// this represents the response for GET DRB info endpoint
type GetDRBInfoResponse struct {
	Status       string            `json:"status"`       // "pending", "in_progress", "installed", "failed", "not_installed"
	Message      string            `json:"message,omitempty"`
	NodeType     string            `json:"nodeType,omitempty"`     // "leader" or "regular"
	Deployment   *DRBDeploymentInfo `json:"deployment,omitempty"`
	FailureReason string           `json:"failureReason,omitempty"`
}
