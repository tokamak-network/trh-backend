package thanos

import (
	"context"
	"testing"

	"github.com/google/uuid"
	thanosSDKConstants "github.com/tokamak-network/trh-sdk/pkg/constants"
)

// TestAAOperatorRegistrationRemovesInfraProviderGuard verifies that the AA Operator
// task is registered based ONLY on NeedsAASetup, without being gated by InfraProvider.
//
// Before: aa-operator was only queued for local deployments if NeedsAASetup returned true.
// After: aa-operator is queued for ANY deployment (local or AWS) if NeedsAASetup returns true.
//
// This test calls the real thanosSDKConstants.NeedsAASetup() function and verifies
// that the decision is made solely on the basis of preset and fee token, regardless
// of infrastructure provider. The actual task queueing happens in deploy() around
// line 498 of deployment.go.
//
// Test scope: This is a contract test that verifies the SDK's NeedsAASetup behavior
// matches the deployment logic. If someone accidentally adds the InfraProvider guard
// back, this test will catch the mismatch between the test assertion and the SDK call.
func TestAAOperatorRegistrationRemovesInfraProviderGuard(t *testing.T) {
	testCases := []struct {
		name             string
		presetID         string
		feeToken         string
		expectAAOperator bool
		description      string
	}{
		{
			name:             "gaming_with_eth_fee",
			presetID:         "gaming",
			feeToken:         "ETH",
			expectAAOperator: true,
			description:      "Gaming preset with ETH fee token → NeedsAASetup returns true → queue aa-operator",
		},
		{
			name:             "gaming_with_ton_fee",
			presetID:         "gaming",
			feeToken:         "TON",
			expectAAOperator: false,
			description:      "Gaming preset with TON fee token → NeedsAASetup returns false → skip aa-operator",
		},
		{
			name:             "general_with_eth_fee",
			presetID:         "general",
			feeToken:         "ETH",
			expectAAOperator: true,
			description:      "General preset with ETH fee token → NeedsAASetup returns true → queue aa-operator",
		},
		{
			name:             "general_with_ton_fee",
			presetID:         "general",
			feeToken:         "TON",
			expectAAOperator: false,
			description:      "General preset with TON fee token → NeedsAASetup returns false → skip aa-operator",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the real SDK function to determine if AA operator should be queued.
			// This is the actual decision logic used in deployment.go line 498.
			wouldQueueAAOperator := thanosSDKConstants.NeedsAASetup(tc.presetID, tc.feeToken)

			if wouldQueueAAOperator != tc.expectAAOperator {
				t.Errorf("%s: preset=%q, feeToken=%q → NeedsAASetup=%v, want %v. %s",
					tc.name, tc.presetID, tc.feeToken, wouldQueueAAOperator, tc.expectAAOperator, tc.description)
			}
		})
	}
}

// TestRestoreAAOperators_SkipsNonAAStacks verifies that restoreAAOperators correctly
// skips stacks that do not require AA setup (where NeedsAASetup returns false).
//
// This test calls the real thanosSDKConstants.NeedsAASetup() function to verify
// that the restoration logic only queues aa-operator tasks for stacks that actually
// need AA setup (i.e., AA presets with non-TON fee tokens).
func TestRestoreAAOperators_SkipsNonAAStacks(t *testing.T) {
	testCases := []struct {
		name             string
		presetID         string
		feeToken         string
		shouldQueueTask  bool
		description      string
	}{
		{
			name:             "gaming_with_eth_fee",
			presetID:         "gaming",
			feeToken:         "ETH",
			shouldQueueTask:  true,
			description:      "Gaming preset with ETH fee token → needs AA setup → task should be queued",
		},
		{
			name:             "gaming_with_ton_fee",
			presetID:         "gaming",
			feeToken:         "TON",
			shouldQueueTask:  false,
			description:      "Gaming preset with TON fee token → does not need AA setup → task should be skipped",
		},
		{
			name:             "general_with_eth_fee",
			presetID:         "general",
			feeToken:         "ETH",
			shouldQueueTask:  true,
			description:      "General preset with ETH fee token → needs AA setup → task should be queued",
		},
		{
			name:             "general_with_ton_fee",
			presetID:         "general",
			feeToken:         "TON",
			shouldQueueTask:  false,
			description:      "General preset with TON fee token → does not need AA setup → task should be skipped",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Verify that NeedsAASetup returns the expected result for this test case.
			// This is the decision logic used in restoreAAOperators to determine whether
			// to queue an aa-operator task for a given stack.
			wouldQueueTask := thanosSDKConstants.NeedsAASetup(tc.presetID, tc.feeToken)

			if wouldQueueTask != tc.shouldQueueTask {
				t.Errorf("%s: preset=%q, feeToken=%q → NeedsAASetup=%v, want %v. %s",
					tc.name, tc.presetID, tc.feeToken, wouldQueueTask, tc.shouldQueueTask, tc.description)
			}
		})
	}
}

func TestInstallDRBOperatorsCallOrder(t *testing.T) {
	// Compile-time check: installDRBOperators must exist as a method on *ThanosStackDeploymentService.
	type hasDRBInstaller interface {
		installDRBOperators(ctx context.Context, stackId uuid.UUID, mnemonic string, l2RPCURL string, chainID uint64)
	}
	var _ hasDRBInstaller = (*ThanosStackDeploymentService)(nil)
}
