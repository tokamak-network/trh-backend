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
			// L1 체인 키가 있는지 확인
			if _, ok := parsed["11155111"]; !ok {
				t.Error("missing Sepolia (11155111) key in L2L1 config")
			}
			// L2 체인 키가 있는지 확인
			if _, ok := parsed["17001"]; !ok {
				t.Error("missing L2 (17001) key in L2L1 config")
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
