package dtos

import (
	"errors"

	"github.com/tokamak-network/trh-sdk/pkg/constants"
)

type BlockExplorerConfig struct {
	APIKey string                      `json:"apiKey"`
	URL    string                      `json:"url"`
	Type   constants.BlockExplorerType `json:"type"`
}

type L1CrossTradeChainInput struct {
	RPC                    string               `json:"rpc" binding:"required" validate:"url"`
	ChainID                uint64               `json:"chainId" binding:"required"`
	PrivateKey             string               `json:"privateKey" binding:"required"`
	IsDeployedNew          bool                 `json:"isDeployedNew" binding:"required" validate:"oneof=true false"`
	DeploymentScriptPath   string               `json:"deploymentScriptPath"`
	ContractName           string               `json:"contractName"`
	BlockExplorerConfig    *BlockExplorerConfig `json:"blockExplorerConfig"`
	CrossTradeProxyAddress string               `json:"crossTradeProxyAddress"`
	CrossTradeAddress      string               `json:"crossTradeAddress"`
}

type L2CrossTradeChainInput struct {
	RPC                     string               `json:"rpc" binding:"required" validate:"url"`
	ChainID                 uint64               `json:"chainId" binding:"required"`
	PrivateKey              string               `json:"privateKey" binding:"required"`
	IsDeployedNew           bool                 `json:"isDeployedNew" binding:"required" validate:"oneof=true false"`
	DeploymentScriptPath    string               `json:"deploymentScriptPath"`
	ContractName            string               `json:"contractName"`
	BlockExplorerConfig     *BlockExplorerConfig `json:"blockExplorerConfig"`
	CrossDomainMessenger    string               `json:"cross_domain_messenger" binding:"required" validate:"eth_address"`
	CrossTradeProxyAddress  string               `json:"crossTradeProxyAddress"`
	CrossTradeAddress       string               `json:"crossTradeAddress"`
	L2Tokens                map[string]string    `json:"l2Tokens"`
	L1Tokens                map[string]string    `json:"l1Tokens"`
	NativeTokenAddressOnL1  string               `json:"nativeTokenAddressOnL1" binding:"required" validate:"eth_address"`
	L1StandardBridgeAddress string               `json:"l1StandardBridgeAddress" binding:"required" validate:"eth_address"`
	L1USDCBridgeAddress     string               `json:"l1USDCBridgeAddress" binding:"required" validate:"eth_address"`
	L1CrossDomainMessenger  string               `json:"l1CrossDomainMessenger" binding:"required" validate:"eth_address"`
}

type InstallCrossChainBridgeRequest struct {
	Mode          constants.CrossTradeDeployMode `json:"mode" binding:"required" validate:"oneof=l2_to_l1 l2_to_l2"`
	ProjectID     string                         `json:"projectID" binding:"required"`
	L1ChainConfig *L1CrossTradeChainInput        `json:"l1ChainConfig" binding:"required"`
	L2ChainConfig []*L2CrossTradeChainInput      `json:"l2ChainConfig" binding:"required"`
}

func (r *InstallCrossChainBridgeRequest) Validate() error {
	if r.Mode != constants.CrossTradeDeployModeL2ToL1 && r.Mode != constants.CrossTradeDeployModeL2ToL2 {
		return errors.New("invalid mode")
	}
	if r.L1ChainConfig == nil {
		return errors.New("l1 chain config is required")
	}
	if r.L2ChainConfig == nil {
		return errors.New("l2 chain config is required")
	}
	if len(r.L2ChainConfig) == 0 {
		return errors.New("l2 chain config is required")
	}
	return nil
}
