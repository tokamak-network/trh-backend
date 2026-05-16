package integrations_test

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/tokamak-network/trh-backend/pkg/services/thanos/integrations"
	thanosTypes "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
)

func TestBuildDAppEnvConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config", ".env.crosstrade")

	cfg := &integrations.CrossTradeDAppConfig{
		L1ChainID:          11155111,
		L2ChainID:          17001,
		L2ChainName:        "Test L2",
		L2RPCURL:           "http://localhost:8545",
		L2BlockExplorerURL: "http://localhost:4001",
		DeployOutput: &thanosTypes.DeployCrossTradeLocalOutput{
			L2CrossTrade:          "0xAAAA0000000000000000000000000000000000AA",
			L2CrossTradeProxy:     "0xBBBB0000000000000000000000000000000000BB",
			L2toL2CrossTradeL2:    "0xCCCC0000000000000000000000000000000000CC",
			L2toL2CrossTradeProxy: "0xDDDD0000000000000000000000000000000000DD",
		},
		L1CrossTradeProxyAddr:  "0xf3473E20F1d9EB4468C72454a27aA1C65B67AB35",
		L2toL2CrossTradeL1Addr: "0xDa2CbF69352cB46d9816dF934402b421d93b6BC2",
	}

	err := integrations.BuildDAppEnvConfig(configPath, cfg)
	if err != nil {
		t.Fatalf("BuildDAppEnvConfig returned error: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("env file not created: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	// 3개 라인: L2L1, L2L2, WALLETCONNECT
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	// NEXT_PUBLIC_CHAIN_CONFIG_L2_L1 검증
	for _, line := range lines {
		if strings.HasPrefix(line, "NEXT_PUBLIC_CHAIN_CONFIG_L2_L1=") {
			jsonStr := strings.TrimPrefix(line, "NEXT_PUBLIC_CHAIN_CONFIG_L2_L1=")
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
				t.Fatalf("NEXT_PUBLIC_CHAIN_CONFIG_L2_L1 is not valid JSON: %v", err)
			}
			if _, ok := parsed["11155111"]; !ok {
				t.Error("missing Sepolia (11155111) key in L2L1 config")
			}
			if _, ok := parsed["17001"]; !ok {
				t.Error("missing L2 (17001) key in L2L1 config")
			}
			// L2 RPC URL must use host.docker.internal (not localhost)
			l2Entry := parsed["17001"].(map[string]interface{})
			if rpcURL, _ := l2Entry["rpc_url"].(string); strings.Contains(rpcURL, "localhost") {
				t.Errorf("L2_L1 L2 rpc_url must not contain localhost, got: %s", rpcURL)
			}
		}
	}

	// NEXT_PUBLIC_CHAIN_CONFIG_L2_L2 검증
	for _, line := range lines {
		if strings.HasPrefix(line, "NEXT_PUBLIC_CHAIN_CONFIG_L2_L2=") {
			jsonStr := strings.TrimPrefix(line, "NEXT_PUBLIC_CHAIN_CONFIG_L2_L2=")
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
				t.Fatalf("NEXT_PUBLIC_CHAIN_CONFIG_L2_L2 is not valid JSON: %v", err)
			}
			if _, ok := parsed["11155111"]; !ok {
				t.Error("missing Sepolia (11155111) key in L2L2 config")
			}
			l2Entry, ok := parsed["17001"].(map[string]interface{})
			if !ok {
				t.Fatal("missing L2 (17001) key in L2L2 config")
			}
			// L2 RPC URL must use host.docker.internal
			if rpcURL, _ := l2Entry["rpc_url"].(string); strings.Contains(rpcURL, "localhost") {
				t.Errorf("L2_L2 L2 rpc_url must not contain localhost, got: %s", rpcURL)
			}
			if rpcURL, _ := l2Entry["rpc_url"].(string); !strings.Contains(rpcURL, "host.docker.internal") {
				t.Errorf("L2_L2 L2 rpc_url must contain host.docker.internal, got: %s", rpcURL)
			}
			// L2 tokens must be an array (not a map) with destination_chains field
			tokens, ok := l2Entry["tokens"].([]interface{})
			if !ok {
				t.Fatalf("L2_L2 L2 tokens must be an array, got: %T", l2Entry["tokens"])
			}
			if len(tokens) == 0 {
				t.Error("L2_L2 L2 tokens array must not be empty")
			}
			firstToken, ok := tokens[0].(map[string]interface{})
			if !ok {
				t.Fatalf("L2_L2 L2 tokens[0] must be an object, got: %T", tokens[0])
			}
			if _, ok := firstToken["destination_chains"]; !ok {
				t.Error("L2_L2 L2 tokens[0] must have destination_chains field")
			}
			// new L2's destination_chains must point to Thanos Sepolia (111551119090), not itself
			destChains, ok := firstToken["destination_chains"].([]interface{})
			if !ok || len(destChains) == 0 {
				t.Error("L2_L2 L2 tokens[0] destination_chains must be a non-empty array")
			} else {
				foundThanos := false
				for _, c := range destChains {
					if uint64(c.(float64)) == 111551119090 {
						foundThanos = true
					}
				}
				if !foundThanos {
					t.Error("new L2 ETH destination_chains must include Thanos Sepolia (111551119090)")
				}
			}

			// Thanos Sepolia (111551119090) must be present in L2L2 config
			thanosEntry, ok := parsed["111551119090"].(map[string]interface{})
			if !ok {
				t.Fatal("missing Thanos Sepolia (111551119090) key in L2L2 config")
			}
			// Thanos Sepolia tokens must be an array (may have empty destination_chains
			// because the Thanos proxy has not registered the new L2 — Thanos→newL2 direction
			// is intentionally disabled to prevent wallet signing failures).
			thanosTokens, ok := thanosEntry["tokens"].([]interface{})
			if !ok || len(thanosTokens) == 0 {
				t.Fatal("Thanos Sepolia tokens must be a non-empty array")
			}
			thanosFirstToken, ok := thanosTokens[0].(map[string]interface{})
			if !ok {
				t.Fatalf("Thanos Sepolia tokens[0] must be an object, got: %T", thanosTokens[0])
			}
			// destination_chains must be an empty array (not absent), confirming the field exists
			// but Thanos→newL2 direction is disabled.
			if _, hasField := thanosFirstToken["destination_chains"]; !hasField {
				t.Error("Thanos Sepolia tokens[0] must have destination_chains field (even if empty)")
			}
			thanosDestChains, ok := thanosFirstToken["destination_chains"].([]interface{})
			if !ok {
				t.Errorf("Thanos Sepolia tokens[0] destination_chains must be an array, got %T", thanosFirstToken["destination_chains"])
			}
			if len(thanosDestChains) != 0 {
				t.Errorf("Thanos Sepolia tokens[0] destination_chains must be empty (Thanos→newL2 disabled), got %v", thanosDestChains)
			}
		}
	}

	// WALLETCONNECT 라인이 있는지 확인
	found := false
	for _, line := range lines {
		if strings.HasPrefix(line, "NEXT_PUBLIC_WALLETCONNECT_PROJECT_ID=") {
			found = true
		}
	}
	if !found {
		t.Error("missing NEXT_PUBLIC_WALLETCONNECT_PROJECT_ID line")
	}
}

func TestBuildDAppEnvConfig_DefiEthPreset(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config", ".env.crosstrade")

	cfg := &integrations.CrossTradeDAppConfig{
		L1ChainID:          11155111,
		L2ChainID:          17001,
		L2ChainName:        "DeFi-ETH L2",
		L2RPCURL:           "http://localhost:8545",
		L2BlockExplorerURL: "http://localhost:4001",
		DeployOutput: &thanosTypes.DeployCrossTradeLocalOutput{
			L2CrossTrade:          "0xAAAA0000000000000000000000000000000000AA",
			L2CrossTradeProxy:     "0xBBBB0000000000000000000000000000000000BB",
			L2toL2CrossTradeL2:    "0xCCCC0000000000000000000000000000000000CC",
			L2toL2CrossTradeProxy: "0xDDDD0000000000000000000000000000000000DD",
		},
		L1CrossTradeProxyAddr:  "0xf3473E20F1d9EB4468C72454a27aA1C65B67AB35",
		L2toL2CrossTradeL1Addr: "0xDa2CbF69352cB46d9816dF934402b421d93b6BC2",
		// defi-eth preset: ETH is the native gas token
		FeeTokenSymbol:      "ETH",
		L2NativeTokenName:   "Ethereum",
		L2NativeTokenSymbol: "ETH",
	}

	err := integrations.BuildDAppEnvConfig(configPath, cfg)
	if err != nil {
		t.Fatalf("BuildDAppEnvConfig returned error: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("env file not created: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	// Check native token metadata is "ETH" (not the TON default) in both L2_L1 and L2_L2
	for _, configKey := range []string{"NEXT_PUBLIC_CHAIN_CONFIG_L2_L1=", "NEXT_PUBLIC_CHAIN_CONFIG_L2_L2="} {
		for _, line := range lines {
			if !strings.HasPrefix(line, configKey) {
				continue
			}
			jsonStr := strings.TrimPrefix(line, configKey)
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
				t.Fatalf("%s is not valid JSON: %v", configKey, err)
			}
			l2Entry, ok := parsed["17001"].(map[string]interface{})
			if !ok {
				t.Fatalf("%s: missing L2 (17001) key", configKey)
			}
			if sym, _ := l2Entry["native_token_symbol"].(string); sym != "ETH" {
				t.Errorf("%s: L2 native_token_symbol want ETH, got %q", configKey, sym)
			}
			if name, _ := l2Entry["native_token_name"].(string); name != "Ethereum" {
				t.Errorf("%s: L2 native_token_name want Ethereum, got %q", configKey, name)
			}
		}
	}
}

func TestBuildDAppEnvConfig_DefaultTonPreset(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config", ".env.crosstrade")

	// Omit L2NativeTokenName/Symbol → should default to "Tokamak Network"/"TON"
	cfg := &integrations.CrossTradeDAppConfig{
		L1ChainID:          11155111,
		L2ChainID:          17001,
		L2ChainName:        "Standard L2",
		L2RPCURL:           "http://localhost:8545",
		L2BlockExplorerURL: "http://localhost:4001",
		DeployOutput: &thanosTypes.DeployCrossTradeLocalOutput{
			L2CrossTrade:          "0xAAAA0000000000000000000000000000000000AA",
			L2CrossTradeProxy:     "0xBBBB0000000000000000000000000000000000BB",
			L2toL2CrossTradeL2:    "0xCCCC0000000000000000000000000000000000CC",
			L2toL2CrossTradeProxy: "0xDDDD0000000000000000000000000000000000DD",
		},
		L1CrossTradeProxyAddr:  "0xf3473E20F1d9EB4468C72454a27aA1C65B67AB35",
		L2toL2CrossTradeL1Addr: "0xDa2CbF69352cB46d9816dF934402b421d93b6BC2",
	}

	err := integrations.BuildDAppEnvConfig(configPath, cfg)
	if err != nil {
		t.Fatalf("BuildDAppEnvConfig returned error: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("env file not created: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	for _, line := range lines {
		if !strings.HasPrefix(line, "NEXT_PUBLIC_CHAIN_CONFIG_L2_L2=") {
			continue
		}
		jsonStr := strings.TrimPrefix(line, "NEXT_PUBLIC_CHAIN_CONFIG_L2_L2=")
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
			t.Fatalf("L2_L2 is not valid JSON: %v", err)
		}
		l2Entry, ok := parsed["17001"].(map[string]interface{})
		if !ok {
			t.Fatal("missing L2 (17001) key")
		}
		if sym, _ := l2Entry["native_token_symbol"].(string); sym != "TON" {
			t.Errorf("default L2 native_token_symbol want TON, got %q", sym)
		}
		if name, _ := l2Entry["native_token_name"].(string); name != "Tokamak Network" {
			t.Errorf("default L2 native_token_name want Tokamak Network, got %q", name)
		}
	}
}

func TestBuildDAppEnvConfig_FeeTokenPresets(t *testing.T) {
	zeroAddr := "0x0000000000000000000000000000000000000000"
	usdcPredeploy := "0x4200000000000000000000000000000000000778"

	cases := []struct {
		feeToken        string
		wantName        string
		wantSymbol      string
		wantNativeAddr  string
		wantHasUSDCERC20 bool
	}{
		{"USDT", "Tether USD", "USDT", zeroAddr, true},
		{"ETH", "Ethereum", "ETH", zeroAddr, true},
		{"TON", "Tokamak Network", "TON", zeroAddr, true},
		{"USDC", "USD Coin", "USDC", zeroAddr, false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.feeToken, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config", ".env.crosstrade")

			cfg := &integrations.CrossTradeDAppConfig{
				L1ChainID:          11155111,
				L2ChainID:          17001,
				L2ChainName:        tc.feeToken + " L2",
				L2RPCURL:           "http://localhost:8545",
				L2BlockExplorerURL: "http://localhost:4001",
				FeeTokenSymbol:     tc.feeToken,
				DeployOutput: &thanosTypes.DeployCrossTradeLocalOutput{
					L2CrossTrade:          "0xAAAA0000000000000000000000000000000000AA",
					L2CrossTradeProxy:     "0xBBBB0000000000000000000000000000000000BB",
					L2toL2CrossTradeL2:    "0xCCCC0000000000000000000000000000000000CC",
					L2toL2CrossTradeProxy: "0xDDDD0000000000000000000000000000000000DD",
				},
				L1CrossTradeProxyAddr:  "0xf3473E20F1d9EB4468C72454a27aA1C65B67AB35",
				L2toL2CrossTradeL1Addr: "0xDa2CbF69352cB46d9816dF934402b421d93b6BC2",
			}

			if err := integrations.BuildDAppEnvConfig(configPath, cfg); err != nil {
				t.Fatalf("BuildDAppEnvConfig(%s): %v", tc.feeToken, err)
			}

			content, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatalf("env file not created: %v", err)
			}
			lines := strings.Split(strings.TrimSpace(string(content)), "\n")

			for _, prefix := range []string{"NEXT_PUBLIC_CHAIN_CONFIG_L2_L1=", "NEXT_PUBLIC_CHAIN_CONFIG_L2_L2="} {
				for _, line := range lines {
					if !strings.HasPrefix(line, prefix) {
						continue
					}
					jsonStr := strings.TrimPrefix(line, prefix)
					var parsed map[string]interface{}
					if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
						t.Fatalf("%s(%s): invalid JSON: %v", prefix, tc.feeToken, err)
					}
					l2Entry, ok := parsed["17001"].(map[string]interface{})
					if !ok {
						t.Fatalf("%s(%s): missing L2 (17001) key", prefix, tc.feeToken)
					}

					// native token metadata
					if sym, _ := l2Entry["native_token_symbol"].(string); sym != tc.wantSymbol {
						t.Errorf("%s(%s): native_token_symbol: want %q, got %q", prefix, tc.feeToken, tc.wantSymbol, sym)
					}
					if name, _ := l2Entry["native_token_name"].(string); name != tc.wantName {
						t.Errorf("%s(%s): native_token_name: want %q, got %q", prefix, tc.feeToken, tc.wantName, name)
					}

					isL2L1 := strings.Contains(prefix, "L2_L1")
					if isL2L1 {
						// L2L1 tokens is a map[string]string
						tokens, ok := l2Entry["tokens"].(map[string]interface{})
						if !ok {
							t.Fatalf("%s(%s): tokens must be an object", prefix, tc.feeToken)
						}
						nativeAddr, _ := tokens[tc.wantSymbol].(string)
						if nativeAddr != tc.wantNativeAddr {
							t.Errorf("%s(%s): tokens[%s]: want %s, got %s", prefix, tc.feeToken, tc.wantSymbol, tc.wantNativeAddr, nativeAddr)
						}
						// USDC ERC20 check: look for USDC entry at the predeploy address specifically.
						usdcERC20Addr, _ := tokens["USDC"].(string)
						hasUSDCERC20 := usdcERC20Addr == usdcPredeploy
						if hasUSDCERC20 && !tc.wantHasUSDCERC20 {
							t.Errorf("%s(%s): tokens must NOT have USDC ERC20 predeploy entry when fee token is USDC", prefix, tc.feeToken)
						}
						if !hasUSDCERC20 && tc.wantHasUSDCERC20 {
							t.Errorf("%s(%s): tokens must have USDC ERC20 entry at %s", prefix, tc.feeToken, usdcPredeploy)
						}
					} else {
						// L2L2 tokens is []l2TokenEntry
						tokenArr, ok := l2Entry["tokens"].([]interface{})
						if !ok || len(tokenArr) == 0 {
							t.Fatalf("%s(%s): tokens must be a non-empty array", prefix, tc.feeToken)
						}
						first, ok := tokenArr[0].(map[string]interface{})
						if !ok {
							t.Fatalf("%s(%s): tokens[0] must be an object", prefix, tc.feeToken)
						}
						if name, _ := first["name"].(string); name != tc.wantSymbol {
							t.Errorf("%s(%s): tokens[0].name: want %q, got %q", prefix, tc.feeToken, tc.wantSymbol, name)
						}
						if addr, _ := first["address"].(string); addr != tc.wantNativeAddr {
							t.Errorf("%s(%s): tokens[0].address: want %s, got %s", prefix, tc.feeToken, tc.wantNativeAddr, addr)
						}
						// USDC ERC20 entry: check for USDC name + predeploy address specifically.
						hasUSDCERC20 := false
						for _, raw := range tokenArr {
							entry, ok := raw.(map[string]interface{})
							if !ok {
								continue
							}
							name, _ := entry["name"].(string)
							addr, _ := entry["address"].(string)
							if name == "USDC" && addr == usdcPredeploy {
								hasUSDCERC20 = true
							}
						}
						if hasUSDCERC20 && !tc.wantHasUSDCERC20 {
							t.Errorf("%s(%s): tokens must NOT have USDC ERC20 predeploy entry when fee token is USDC", prefix, tc.feeToken)
						}
						if !hasUSDCERC20 && tc.wantHasUSDCERC20 {
							t.Errorf("%s(%s): tokens must have USDC ERC20 entry at %s", prefix, tc.feeToken, usdcPredeploy)
						}
					}
				}
			}
		})
	}
}

func TestDeployL1UsdcBridgeAdapterBytecodeEncoding(t *testing.T) {
	// Verify the adapter initCode (bytecode + constructor arg) is well-formed.
	// Constructor arg: 32-byte left-padded address.
	const adapterBytecodeHex = "60a034606d57601f6104f438819003918201601f19168301916001600160401b03831184841017607157808492602094604052833981010312606d57516001600160a01b0381168103606d5760805260405161046e9081610086823960805181818161012601526102b30152f35b5f80fd5b634e487b7160e01b5f52604160045260245ffdfe6080806040526004361015610012575f80fd5b5f905f3560e01c908163385a6970146102a1575063540abf7314610034575f80fd5b346102205760c0366003190112610220576004356001600160a01b03811690819003610220576024356001600160a01b03811690819003610220576044356001600160a01b0381169081900361022057606435926084359263ffffffff84168094036102205760a43567ffffffffffffffff811161022057366023820112156102205780600401359467ffffffffffffffff8611610220573660248784010111610220576101126040516323b872dd60e01b60208201523360248201523060448201528860648201526064815261010c6084826102e2565b85610330565b60405163095ea7b360e01b602082019081527f00000000000000000000000000000000000000000000000000000000000000006001600160a01b03166024830181905260448084018b905283529591905f9081906101716064856102e2565b83519082865af161018061039b565b81610272575b5080610268575b15610224575b50843b1561022057865f976024899560e4956040519c8d9b8c9a8b9863041c592960e51b8a5260048a01528589015260448801526064870152608486015260c060a48601528260c486015201848401378181018301849052601f01601f191681010301925af1801561021515610207575080f35b61021391505f906102e2565b005b6040513d5f823e3d90fd5b5f80fd5b6102629061025c60405163095ea7b360e01b60208201528860248201525f6044820152604481526102566064826102e2565b84610330565b82610330565b5f610193565b50813b151561018d565b8051801592508215610287575b50505f610186565b61029a9250602080918301019101610318565b5f8061027f565b34610220575f366003190112610220577f00000000000000000000000000000000000000000000000000000000000000006001600160a01b03168152602090f35b90601f8019910116810190811067ffffffffffffffff82111761030457604052565b634e487b7160e01b5f52604160045260245ffd5b90816020910312610220575180151581036102205790565b5f806103589260018060a01b03169360208151910182865af161035161039b565b90836103da565b8051908115159182610380575b505061036e5750565b635274afe760e01b5f5260045260245ffd5b6103939250602080918301019101610318565b155f80610365565b3d156103d5573d9067ffffffffffffffff821161030457604051916103ca601f8201601f1916602001846102e2565b82523d5f602084013e565b606090565b906103fe57508051156103ef57805190602001fd5b630a12f52160e11b5f5260045ffd5b8151158061042f575b61040f575090565b639996b31560e01b5f9081526001600160a01b0391909116600452602490fd5b50803b1561040756fea264697066735822122043099adb46c82344de445ae6fa40265a6265f7ef3b397d6af9c1e68210d0c28f64736f6c63430008210033"

	bytecodeBytes, err := hex.DecodeString(adapterBytecodeHex)
	if err != nil {
		t.Fatalf("bytecode hex decode failed: %v", err)
	}
	if len(bytecodeBytes) == 0 {
		t.Fatal("bytecode is empty")
	}

	bridgeAddr := common.HexToAddress("0xABCD000000000000000000000000000000001234")
	constructorArg := make([]byte, 32)
	copy(constructorArg[12:32], bridgeAddr.Bytes()) // left-pad to 32 bytes

	initCode := append(bytecodeBytes, constructorArg...)
	if len(initCode) != len(bytecodeBytes)+32 {
		t.Fatalf("initCode length mismatch: got %d", len(initCode))
	}
	// address should appear at the last 20 bytes of the constructor arg slot
	gotAddr := common.BytesToAddress(initCode[len(bytecodeBytes)+12:])
	if gotAddr != bridgeAddr {
		t.Errorf("constructor arg address mismatch: got %s, want %s", gotAddr.Hex(), bridgeAddr.Hex())
	}
}
