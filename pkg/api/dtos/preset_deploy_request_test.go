package dtos_test

import (
	"encoding/json"
	"testing"

	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
)

// TestPresetDeployRequest_EnableFaultProof_Default verifies that
// omitting enableFaultProof in the JSON body results in false (Go zero value).
// This prevents accidental fault proof activation on presets that don't declare it.
func TestPresetDeployRequest_EnableFaultProof_Default(t *testing.T) {
	raw := `{"presetId":"general","chainName":"my-chain","network":"Testnet","seedPhrase":"a","infraProvider":"aws"}`
	var req dtos.PresetDeployRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if req.EnableFaultProof {
		t.Error("EnableFaultProof should default to false when omitted from JSON")
	}
}

// TestPresetDeployRequest_EnableFaultProof_True verifies that the field is
// correctly deserialized when set to true (Full preset path).
func TestPresetDeployRequest_EnableFaultProof_True(t *testing.T) {
	raw := `{"presetId":"full","chainName":"my-chain","network":"Testnet","seedPhrase":"a","infraProvider":"aws","enableFaultProof":true}`
	var req dtos.PresetDeployRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if !req.EnableFaultProof {
		t.Error("EnableFaultProof should be true when set in JSON")
	}
}

// TestPresetDeployRequest_EnableFaultProof_False verifies explicit false round-trips.
func TestPresetDeployRequest_EnableFaultProof_False(t *testing.T) {
	raw := `{"presetId":"gaming","chainName":"my-chain","network":"Testnet","seedPhrase":"a","infraProvider":"aws","enableFaultProof":false}`
	var req dtos.PresetDeployRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if req.EnableFaultProof {
		t.Error("EnableFaultProof should be false when explicitly set to false in JSON")
	}
}

// TestPresetDeployRequest_EnableFaultProof_RoundTrip verifies JSON marshal→unmarshal
// preserves the true value (regression guard for field tag correctness).
func TestPresetDeployRequest_EnableFaultProof_RoundTrip(t *testing.T) {
	original := dtos.PresetDeployRequest{
		PresetID:         "full",
		ChainName:        "my-chain",
		Network:          "Testnet",
		SeedPhrase:       "a",
		InfraProvider:    "aws",
		EnableFaultProof: true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded dtos.PresetDeployRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if !decoded.EnableFaultProof {
		t.Errorf("EnableFaultProof lost after JSON round-trip; marshaled: %s", string(data))
	}
}
