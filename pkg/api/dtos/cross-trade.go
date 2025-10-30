package dtos

import (
	"errors"

	"github.com/tokamak-network/trh-sdk/pkg/constants"
)

type BlockExplorerConfig struct {
	APIKey string                      `json:"api_key"`
	URL    string                      `json:"url"`
	Type   constants.BlockExplorerType `json:"type"`
}

type L1CrossTradeChainInput struct {
	RPC                    string               `json:"rpc"`
	ChainID                uint64               `json:"chain_id"`
	PrivateKey             string               `json:"private_key"`
	IsDeployedNew          bool                 `json:"is_deployed_new"`
	DeploymentScriptPath   string               `json:"deployment_script_path"`
	ContractName           string               `json:"contract_name"`
	BlockExplorerConfig    *BlockExplorerConfig `json:"block_explorer_config"`
	CrossTradeProxyAddress string               `json:"cross_trade_proxy_address"`
	CrossTradeAddress      string               `json:"cross_trade_address"`
}

type L2CrossTradeChainInput struct {
	RPC                    string               `json:"rpc"`
	ChainID                uint64               `json:"chain_id"`
	PrivateKey             string               `json:"private_key"`
	IsDeployedNew          bool                 `json:"is_deployed_new"`
	DeploymentScriptPath   string               `json:"deployment_script_path"`
	ContractName           string               `json:"contract_name"`
	BlockExplorerConfig    *BlockExplorerConfig `json:"block_explorer_config"`
	CrossDomainMessenger   string               `json:"cross_domain_messenger"`
	CrossTradeProxyAddress string               `json:"cross_trade_proxy_address"`
	CrossTradeAddress      string               `json:"cross_trade_address"`
	USDTAddress            string               `json:"usdt_address"`
	USDCAddress            string               `json:"usdc_address"`
	TONAddress             string               `json:"ton_address"`
}

type InstallCrossChainBridgeRequest struct {
	Mode          constants.CrossTradeDeployMode `json:"mode" binding:"required" validate:"oneof=l2_to_l1 l2_to_l2"`
	ProjectID     string                         `json:"project_id" binding:"required"`
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
