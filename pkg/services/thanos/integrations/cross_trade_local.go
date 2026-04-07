package integrations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	thanosTypes "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
)

// CrossTradeDAppConfig holds all configuration needed to generate the .env.crosstrade file.
type CrossTradeDAppConfig struct {
	L1ChainID              uint64
	L2ChainID              uint64
	L2ChainName            string
	L2RPCURL               string
	L2BlockExplorerURL     string
	DeployOutput           *thanosTypes.DeployCrossTradeLocalOutput
	L1CrossTradeProxyAddr  string
	L2toL2CrossTradeL1Addr string
}

// chainConfigEntry is the per-chain JSON structure for NEXT_PUBLIC_CHAIN_CONFIG_* env vars.
type chainConfigEntry struct {
	Name              string            `json:"name"`
	DisplayName       string            `json:"display_name"`
	NativeTokenName   string            `json:"native_token_name"`
	NativeTokenSymbol string            `json:"native_token_symbol"`
	RPCURL            string            `json:"rpc_url"`
	BlockExplorerURL  string            `json:"block_explorer_url"`
	Contracts         map[string]string `json:"contracts"`
	Tokens            map[string]string `json:"tokens"`
}

// BuildDAppEnvConfig generates config/.env.crosstrade for the CrossTrade dApp container (BE-07).
// configPath: absolute or relative path to the output file (e.g. "config/.env.crosstrade").
func BuildDAppEnvConfig(configPath string, cfg *CrossTradeDAppConfig) error {
	l2ChainIDStr := fmt.Sprintf("%d", cfg.L2ChainID)
	l1ChainIDStr := fmt.Sprintf("%d", cfg.L1ChainID)

	sepoliaTokens := map[string]string{
		"ETH":  "0x0000000000000000000000000000000000000000",
		"USDC": "0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238",
		"USDT": "",
		"TON":  "",
	}
	l2Tokens := map[string]string{
		"ETH":  "0x0000000000000000000000000000000000000000",
		"USDC": "0x4200000000000000000000000000000000000778", // L2 USDC predeploy
		"USDT": "",
		"TON":  "",
	}

	// NEXT_PUBLIC_CHAIN_CONFIG_L2_L1:
	// L1 side: l1_cross_trade = L1CrossTradeProxy address
	// L2 side: l2_cross_trade = L2CrossTradeProxy address
	l2l1Config := map[string]chainConfigEntry{
		l1ChainIDStr: {
			Name:              "Ethereum Sepolia",
			DisplayName:       "Ethereum",
			NativeTokenName:   "Ethereum",
			NativeTokenSymbol: "ETH",
			RPCURL:            "",
			BlockExplorerURL:  "https://sepolia.etherscan.io",
			Contracts:         map[string]string{"l1_cross_trade": cfg.L1CrossTradeProxyAddr},
			Tokens:            sepoliaTokens,
		},
		l2ChainIDStr: {
			Name:              cfg.L2ChainName,
			DisplayName:       cfg.L2ChainName,
			NativeTokenName:   "Tokamak Network",
			NativeTokenSymbol: "TON",
			RPCURL:            cfg.L2RPCURL,
			BlockExplorerURL:  cfg.L2BlockExplorerURL,
			Contracts:         map[string]string{"l2_cross_trade": cfg.DeployOutput.L2CrossTradeProxy},
			Tokens:            l2Tokens,
		},
	}

	// NEXT_PUBLIC_CHAIN_CONFIG_L2_L2:
	// L1 side: l1_cross_trade = L2toL2CrossTradeL1 address (different from L2L1 config!)
	// L2 side: l2_cross_trade = L2toL2CrossTradeProxy address
	l2l2Config := map[string]chainConfigEntry{
		l1ChainIDStr: {
			Name:              "Ethereum Sepolia",
			DisplayName:       "Ethereum",
			NativeTokenName:   "Ethereum",
			NativeTokenSymbol: "ETH",
			RPCURL:            "",
			BlockExplorerURL:  "https://sepolia.etherscan.io",
			Contracts:         map[string]string{"l1_cross_trade": cfg.L2toL2CrossTradeL1Addr},
			Tokens:            sepoliaTokens,
		},
		l2ChainIDStr: {
			Name:              cfg.L2ChainName,
			DisplayName:       cfg.L2ChainName,
			NativeTokenName:   "Tokamak Network",
			NativeTokenSymbol: "TON",
			RPCURL:            cfg.L2RPCURL,
			BlockExplorerURL:  cfg.L2BlockExplorerURL,
			Contracts:         map[string]string{"l2_cross_trade": cfg.DeployOutput.L2toL2CrossTradeProxy},
			Tokens:            l2Tokens,
		},
	}

	l2l1JSON, err := json.Marshal(l2l1Config)
	if err != nil {
		return fmt.Errorf("marshal L2L1 chain config: %w", err)
	}
	l2l2JSON, err := json.Marshal(l2l2Config)
	if err != nil {
		return fmt.Errorf("marshal L2L2 chain config: %w", err)
	}

	content := fmt.Sprintf(
		"NEXT_PUBLIC_CHAIN_CONFIG_L2_L1=%s\nNEXT_PUBLIC_CHAIN_CONFIG_L2_L2=%s\nNEXT_PUBLIC_WALLETCONNECT_PROJECT_ID=\n",
		string(l2l1JSON),
		string(l2l2JSON),
	)

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write .env.crosstrade: %w", err)
	}
	return nil
}
