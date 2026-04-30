package integrations_test

import (
	"testing"

	integrations "github.com/tokamak-network/trh-backend/pkg/services/thanos/integrations"
	thanosConstants "github.com/tokamak-network/trh-sdk/pkg/constants"
)

func TestBuildDefaultCrossTradeL2L1RequestPrivateKeyNormalization(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"abcdef", "0xabcdef"},
		{"0xabcdef", "0xabcdef"},
		{"a", "0xa"}, // len=1 must not panic
	}
	for _, tc := range cases {
		req := integrations.BuildDefaultCrossTradeL2L1Request("https://l1.rpc", 11155111, "https://l2.rpc", 12345, tc.input, "proj")
		if req.L1ChainConfig.PrivateKey != tc.want {
			t.Errorf("input %q: got %q, want %q", tc.input, req.L1ChainConfig.PrivateKey, tc.want)
		}
	}
}

func TestBuildDefaultCrossTradeL2L1Request(t *testing.T) {
	l1RPC := "https://sepolia.rpc"
	l1ChainID := uint64(11155111)
	l2RPC := "https://l2.rpc"
	l2ChainID := uint64(12345)
	privateKey := "0xdeadbeef"
	projectID := "test-project"

	req := integrations.BuildDefaultCrossTradeL2L1Request(l1RPC, l1ChainID, l2RPC, l2ChainID, privateKey, projectID)

	if req.L1ChainConfig == nil {
		t.Fatal("L1ChainConfig is nil")
	}
	if req.L1ChainConfig.RPC != l1RPC {
		t.Errorf("L1ChainConfig.RPC = %q, want %q", req.L1ChainConfig.RPC, l1RPC)
	}
	if !req.L1ChainConfig.IsDeployedNew {
		t.Error("L1ChainConfig.IsDeployedNew should be true (scratch mode)")
	}
	if len(req.L2ChainConfig) != 1 {
		t.Errorf("L2ChainConfig len = %d, want 1", len(req.L2ChainConfig))
	}
	if req.L2ChainConfig[0].RPC != l2RPC {
		t.Errorf("L2ChainConfig[0].RPC = %q, want %q", req.L2ChainConfig[0].RPC, l2RPC)
	}
	if req.L2ChainConfig[0].CrossDomainMessenger != "0x4200000000000000000000000000000000000007" {
		t.Errorf("CDM = %q, want predeploy address", req.L2ChainConfig[0].CrossDomainMessenger)
	}
	if req.Mode != thanosConstants.CrossTradeDeployModeL2ToL1 {
		t.Errorf("Mode = %q, want %q", req.Mode, thanosConstants.CrossTradeDeployModeL2ToL1)
	}
}

func TestBuildDefaultCrossTradeL2L2Request(t *testing.T) {
	l1RPC := "https://sepolia.rpc"
	l1ChainID := uint64(11155111)
	privateKey := "0xdeadbeef"
	projectID := "test-project"

	req := integrations.BuildDefaultCrossTradeL2L2Request(l1RPC, l1ChainID, privateKey, projectID)

	if len(req.L2ChainConfig) != 3 {
		t.Errorf("L2ChainConfig len = %d, want 3", len(req.L2ChainConfig))
	}
	chainIDs := map[uint64]bool{}
	for _, c := range req.L2ChainConfig {
		chainIDs[c.ChainID] = true
		// Each L2 chain should have addresses from DefaultContractAddresses
		if _, exists := thanosConstants.DefaultContractAddresses[c.ChainID]; !exists {
			t.Errorf("chain ID %d not found in DefaultContractAddresses", c.ChainID)
		}
	}
	for _, expected := range []uint64{
		thanosConstants.OptimismSepoliaChainID,
		thanosConstants.BaseSepoliaChainID,
		thanosConstants.UnichainSepoliaChainID,
	} {
		if !chainIDs[expected] {
			t.Errorf("missing chainID %d in L2ChainConfig", expected)
		}
	}
	if req.Mode != thanosConstants.CrossTradeDeployModeL2ToL2 {
		t.Errorf("Mode = %q, want %q", req.Mode, thanosConstants.CrossTradeDeployModeL2ToL2)
	}
}
