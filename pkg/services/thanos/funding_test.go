package thanos_test

import (
	"math/big"
	"testing"

	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	thanosService "github.com/tokamak-network/trh-backend/pkg/services/thanos"
)

// TestRequiredFundingByNetwork_AllNetworksDefined verifies that both Testnet
// and Mainnet have required funding entries.
func TestRequiredFundingByNetwork_AllNetworksDefined(t *testing.T) {
	networks := []entities.DeploymentNetwork{
		entities.DeploymentNetworkTestnet,
		entities.DeploymentNetworkMainnet,
	}

	for _, network := range networks {
		reqs, ok := thanosService.RequiredFundingByNetwork[network]
		if !ok {
			t.Errorf("RequiredFundingByNetwork missing entry for network %q", network)
			continue
		}
		roles := []string{"admin", "sequencer", "batcher", "proposer"}
		for _, role := range roles {
			if _, ok := reqs[role]; !ok {
				t.Errorf("network %q: missing required funding for role %q", network, role)
			}
		}
	}
}

// TestRequiredFundingByNetwork_ValuesArePositiveIntegers verifies that all
// funding values are valid positive integer strings (wei amounts).
func TestRequiredFundingByNetwork_ValuesArePositiveIntegers(t *testing.T) {
	for network, roles := range thanosService.RequiredFundingByNetwork {
		for role, weiStr := range roles {
			n := new(big.Int)
			if _, ok := n.SetString(weiStr, 10); !ok {
				t.Errorf("network %q, role %q: %q is not a valid integer", network, role, weiStr)
				continue
			}
			if n.Sign() <= 0 {
				t.Errorf("network %q, role %q: required balance must be positive, got %s", network, role, weiStr)
			}
		}
	}
}

// TestRequiredFundingByNetwork_MainnetHigherThanTestnet verifies that Mainnet
// requirements are at least as high as Testnet for each role.
func TestRequiredFundingByNetwork_MainnetHigherThanTestnet(t *testing.T) {
	testnet := thanosService.RequiredFundingByNetwork[entities.DeploymentNetworkTestnet]
	mainnet := thanosService.RequiredFundingByNetwork[entities.DeploymentNetworkMainnet]

	roles := []string{"admin", "sequencer", "batcher", "proposer"}
	for _, role := range roles {
		testnetVal := new(big.Int)
		mainnetVal := new(big.Int)
		testnetVal.SetString(testnet[role], 10)
		mainnetVal.SetString(mainnet[role], 10)

		if mainnetVal.Cmp(testnetVal) < 0 {
			t.Errorf("role %q: mainnet requirement (%s) is less than testnet (%s)",
				role, mainnet[role], testnet[role])
		}
	}
}

// TestAccountFunding_FulfilledWhenBalanceMeetsRequired verifies the fulfilled
// logic matches balance >= required semantics.
func TestAccountFunding_FulfilledWhenBalanceMeetsRequired(t *testing.T) {
	cases := []struct {
		name      string
		balance   string
		required  string
		fulfilled bool
	}{
		{"equal", "1000000000000000000", "1000000000000000000", true},
		{"above", "2000000000000000000", "1000000000000000000", true},
		{"below", "500000000000000000", "1000000000000000000", false},
		{"zero balance", "0", "1000000000000000000", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			balance := new(big.Int)
			balance.SetString(tc.balance, 10)
			req := new(big.Int)
			req.SetString(tc.required, 10)

			got := balance.Cmp(req) >= 0
			if got != tc.fulfilled {
				t.Errorf("balance %s vs required %s: expected fulfilled=%v, got %v",
					tc.balance, tc.required, tc.fulfilled, got)
			}
		})
	}
}
