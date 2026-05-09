package thanos

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	thanosStack "github.com/tokamak-network/trh-sdk/pkg/stacks/thanos"
)

func TestBuildLocalChainInformation_ReadsRollupJSON(t *testing.T) {
	dir := t.TempDir()

	rollup := map[string]any{
		"l1_chain_id": 11155111,
		"l2_chain_id": 111551115,
	}
	data, err := json.Marshal(rollup)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rollup.json"), data, 0o644))

	info := BuildLocalChainInformation(dir)

	require.Equal(t, 11155111, info.L1ChainID)
	require.Equal(t, 111551115, info.L2ChainID)
	require.Equal(t, dir+"/rollup.json", info.RollupFilePath)
}

func TestBuildLocalChainInformation_MissingFile(t *testing.T) {
	dir := t.TempDir()

	info := BuildLocalChainInformation(dir)

	require.Equal(t, 0, info.L1ChainID)
	require.Equal(t, 0, info.L2ChainID)
}

// TestAutoInstallCrossTradeAWS_WrapperExists is a compile-time guard that verifies:
// (1) AutoInstallCrossTradeAWS wrapper exists in this package,
// (2) its signature returns the SDK output struct (not the broken forge-based output),
// (3) callers can extract both dApp URLs and all four contract addresses from the output.
//
// This test was written to prevent regression to the forge path, which requires L2 ETH
// that the deployer never has on a freshly-deployed chain.
func TestAutoInstallCrossTradeAWS_WrapperExists(t *testing.T) {
	// compile-time: AutoInstallCrossTradeAWS must accept an SDK ThanosStack and return the deposit-tx output.
	var _ func(context.Context, *thanosStack.ThanosStack) (*thanosStack.AutoInstallCrossTradeAWSOutput, error) = AutoInstallCrossTradeAWS

	// compile-time: all required fields must be present on the output struct.
	var out thanosStack.AutoInstallCrossTradeAWSOutput
	_ = out.L2CrossTradeProxy
	_ = out.L2toL2CrossTradeProxy
	_ = out.L1CrossTradeProxy
	_ = out.L2toL2CrossTradeL1
	_ = out.L2L1DAppURL
	_ = out.L2L2DAppURL
}
