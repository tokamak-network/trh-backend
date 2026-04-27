package thanos

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
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
