package integrations_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
			// Thanos Sepolia tokens must be an array with destination_chains pointing to new L2
			thanosTokens, ok := thanosEntry["tokens"].([]interface{})
			if !ok || len(thanosTokens) == 0 {
				t.Fatal("Thanos Sepolia tokens must be a non-empty array")
			}
			thanosFirstToken, ok := thanosTokens[0].(map[string]interface{})
			if !ok {
				t.Fatalf("Thanos Sepolia tokens[0] must be an object, got: %T", thanosTokens[0])
			}
			thanosDestChains, ok := thanosFirstToken["destination_chains"].([]interface{})
			if !ok || len(thanosDestChains) == 0 {
				t.Error("Thanos Sepolia tokens[0] destination_chains must be a non-empty array")
			} else {
				foundNewL2 := false
				for _, c := range thanosDestChains {
					if uint64(c.(float64)) == 17001 {
						foundNewL2 = true
					}
				}
				if !foundNewL2 {
					t.Error("Thanos Sepolia token destination_chains must include new L2 (17001)")
				}
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
