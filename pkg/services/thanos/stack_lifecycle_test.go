package thanos_test

import (
	"testing"

	"github.com/tokamak-network/trh-backend/pkg/services/thanos/presets"
)

// TestLocalDeploymentCrossTradeEntityCreation verifies that crossTrade integration
// entities are created for DeFi/Full presets on local deployment, and not for
// Gaming/General presets.
//
// This test simulates the integration entity creation loop in stack_lifecycle.go,
// assuming crossTrade is NOT in localUnsupported (after BE-01 fix).
//
// TDD RED: DeFi_local_creates_crossTrade_entity fails because
// DefaultPresetDefinitions["defi"].Modules["crossTrade"] is currently false.
// TDD GREEN: All subtests pass after Plan 02-02 aligns preset definitions.
func TestLocalDeploymentCrossTradeEntityCreation(t *testing.T) {
	// Simulate the localUnsupported map from stack_lifecycle.go after BE-01 fix.
	// crossTrade should NOT be in localUnsupported after the fix.
	localUnsupported := map[string]bool{
		// crossTrade intentionally absent: local deployment must support it
	}

	testCases := []struct {
		name             string
		preset           string
		expectCrossTrade bool
	}{
		{"DeFi_local_creates_crossTrade_entity", "defi", true},
		{"Full_local_creates_crossTrade_entity", "full", true},
		{"Gaming_local_no_crossTrade_entity", "gaming", false},
		{"General_local_no_crossTrade_entity", "general", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			def := presets.DefaultPresetDefinitions[tc.preset]
			crossTradeEnabled := def.Modules["crossTrade"]
			isLocal := true
			blocked := isLocal && localUnsupported["crossTrade"]

			wouldCreate := crossTradeEnabled && !blocked
			if wouldCreate != tc.expectCrossTrade {
				t.Errorf("preset %q: crossTrade entity creation = %v, want %v (enabled=%v, blocked=%v)",
					tc.preset, wouldCreate, tc.expectCrossTrade, crossTradeEnabled, blocked)
			}
		})
	}
}

// TestLocalUnsupportedNoCrossTrade verifies that the DeFi preset has crossTrade=true,
// which is the precondition for BE-01 (removing crossTrade from localUnsupported).
//
// TDD RED: Fails because DefaultPresetDefinitions["defi"].Modules["crossTrade"] is
// currently false in the Backend preset definitions.
func TestLocalUnsupportedNoCrossTrade(t *testing.T) {
	def := presets.DefaultPresetDefinitions["defi"]
	if !def.Modules["crossTrade"] {
		t.Errorf("DefaultPresetDefinitions[%q].Modules[crossTrade] = false, want true (BE-01 precondition)", "defi")
	}
}
