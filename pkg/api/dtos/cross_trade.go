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
	ChainName              string               `json:"chainName" binding:"required"`
	IsDeployedNew          bool                 `json:"isDeployedNew" binding:"required" validate:"oneof=true false"`
	BlockExplorerConfig    *BlockExplorerConfig `json:"blockExplorerConfig"`
	CrossTradeProxyAddress string               `json:"crossTradeProxyAddress"`
	CrossTradeAddress      string               `json:"crossTradeAddress"`
}

type L2CrossTradeChainInput struct {
	RPC                     string               `json:"rpc" binding:"required" validate:"url"`
	ChainID                 uint64               `json:"chainId" binding:"required"`
	PrivateKey              string               `json:"privateKey" binding:"required"`
	IsDeployedNew           bool                 `json:"isDeployedNew" binding:"required" validate:"oneof=true false"`
	ChainName               string               `json:"chainName" binding:"required"`
	BlockExplorerConfig     *BlockExplorerConfig `json:"blockExplorerConfig"`
	CrossDomainMessenger    string               `json:"crossDomainMessenger" binding:"required" validate:"eth_address"`
	CrossTradeProxyAddress  string               `json:"crossTradeProxyAddress"`
	NativeTokenAddress      string               `json:"nativeTokenAddress" binding:"required" validate:"eth_address"`
	CrossTradeAddress       string               `json:"crossTradeAddress"`
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

type L2TokenInput struct {
	ChainID      uint64 `json:"chainId" binding:"required"`
	TokenAddress string `json:"tokenAddress" binding:"required" validate:"eth_address"`
}

type RegisterTokensRequest struct {
	TokenName      string          `json:"tokenName" binding:"required"`
	L1TokenAddress string          `json:"l1TokenAddress" binding:"required" validate:"eth_address"`
	L2TokenInputs  []*L2TokenInput `json:"l2TokenInputs" binding:"required"`
}

type DeployNewL2ChainRequest struct {
	Mode          constants.CrossTradeDeployMode `json:"mode" binding:"required" validate:"oneof=l2_to_l1 l2_to_l2"`
	L2ChainConfig *L2CrossTradeChainInput        `json:"l2ChainConfig" binding:"required"`
}

func (r *DeployNewL2ChainRequest) Validate() error {
	if r.Mode != constants.CrossTradeDeployModeL2ToL1 && r.Mode != constants.CrossTradeDeployModeL2ToL2 {
		return errors.New("invalid mode")
	}
	if r.L2ChainConfig == nil {
		return errors.New("l2 chain config is required")
	}
	return nil
}

type RegisterTokensAPIRequest struct {
	Mode   constants.CrossTradeDeployMode `json:"mode" binding:"required" validate:"oneof=l2_to_l1 l2_to_l2"`
	Tokens []*RegisterTokensRequest       `json:"tokens" binding:"required"`
}

func (r *RegisterTokensAPIRequest) Validate() error {
	if r.Mode != constants.CrossTradeDeployModeL2ToL1 && r.Mode != constants.CrossTradeDeployModeL2ToL2 {
		return errors.New("invalid mode")
	}
	if len(r.Tokens) == 0 {
		return errors.New("tokens are required")
	}

	seenL2ChainIDs := make(map[uint64]bool)
	for _, token := range r.Tokens {
		if token.TokenName == "" {
			return errors.New("token name is required")
		}
		if token.L1TokenAddress == "" {
			return errors.New("l1 token address is required")
		}
		if len(token.L2TokenInputs) == 0 {
			return errors.New("l2 token inputs are required")
		}
		for _, l2TokenInput := range token.L2TokenInputs {
			if l2TokenInput.ChainID == 0 {
				return errors.New("chain id is required")
			}
			if l2TokenInput.TokenAddress == "" {
				return errors.New("token address is required")
			}
			if seenL2ChainIDs[l2TokenInput.ChainID] {
				return errors.New("chain id already exists")
			}
			seenL2ChainIDs[l2TokenInput.ChainID] = true
		}
	}
	return nil
}
