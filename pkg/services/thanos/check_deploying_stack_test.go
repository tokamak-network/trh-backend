package thanos

// Tests for checkNoActiveDeployingStack — the guard that prevents concurrent
// deployments to shared L1 networks (Testnet/Mainnet) which would cause nonce
// conflicts when the same admin key is used for multiple simultaneous L1 txs.

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

// makeDeployingStack builds a StackEntity with the given network, provider, and status.
func makeDeployingStack(t *testing.T, network entities.DeploymentNetwork, provider string, status entities.StackStatus) *entities.StackEntity {
	t.Helper()
	cfg := dtos.DeployThanosRequest{InfraProvider: provider}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	return &entities.StackEntity{
		ID:      uuid.New(),
		Status:  status,
		Network: network,
		Config:  raw,
	}
}

func TestCheckNoActiveDeployingStack_NoStacks(t *testing.T) {
	svc := svcWithStacks(t, nil)
	if resp := svc.checkNoActiveDeployingStack(); resp != nil {
		t.Errorf("expected nil when no stacks, got %+v", resp)
	}
}

func TestCheckNoActiveDeployingStack_TestnetPendingBlocks(t *testing.T) {
	st := makeDeployingStack(t, entities.DeploymentNetworkTestnet, "aws", entities.StackStatusPending)
	svc := svcWithStacks(t, []*entities.StackEntity{st})
	resp := svc.checkNoActiveDeployingStack()
	if resp == nil {
		t.Fatal("expected 409, got nil")
	}
	if resp.Status != http.StatusConflict {
		t.Errorf("expected HTTP 409, got %d", resp.Status)
	}
}

func TestCheckNoActiveDeployingStack_TestnetDeployingBlocks(t *testing.T) {
	st := makeDeployingStack(t, entities.DeploymentNetworkTestnet, "aws", entities.StackStatusDeploying)
	svc := svcWithStacks(t, []*entities.StackEntity{st})
	resp := svc.checkNoActiveDeployingStack()
	if resp == nil {
		t.Fatal("expected 409, got nil")
	}
	if resp.Status != http.StatusConflict {
		t.Errorf("expected HTTP 409, got %d", resp.Status)
	}
}

func TestCheckNoActiveDeployingStack_MainnetDeployingBlocks(t *testing.T) {
	st := makeDeployingStack(t, entities.DeploymentNetworkMainnet, "aws", entities.StackStatusDeploying)
	svc := svcWithStacks(t, []*entities.StackEntity{st})
	resp := svc.checkNoActiveDeployingStack()
	if resp == nil {
		t.Fatal("expected 409, got nil")
	}
	if resp.Status != http.StatusConflict {
		t.Errorf("expected HTTP 409, got %d", resp.Status)
	}
}

func TestCheckNoActiveDeployingStack_LocalDeployingBlocks(t *testing.T) {
	// local infra + Testnet (not LocalDevnet) must also be blocked.
	st := makeDeployingStack(t, entities.DeploymentNetworkTestnet, "local", entities.StackStatusDeploying)
	// Give it a non-empty DeploymentPath so Docker reconcile path is taken —
	// but since no real containers exist in the test, Docker check returns false
	// and the stack would be auto-corrected. To keep the test focused on the
	// network guard (not Docker reconcile), use an empty DeploymentPath so the
	// reconcile branch is skipped.
	st.DeploymentPath = ""
	svc := svcWithStacks(t, []*entities.StackEntity{st})
	resp := svc.checkNoActiveDeployingStack()
	if resp == nil {
		t.Fatal("expected 409 for local+Testnet deploying, got nil")
	}
	if resp.Status != http.StatusConflict {
		t.Errorf("expected HTTP 409, got %d", resp.Status)
	}
}

func TestCheckNoActiveDeployingStack_LocalDevnetIgnored(t *testing.T) {
	// LocalDevnet uses an isolated local L1 node — no nonce conflict with Testnet/Mainnet.
	st := makeDeployingStack(t, entities.DeploymentNetworkLocalDevnet, "local", entities.StackStatusDeploying)
	svc := svcWithStacks(t, []*entities.StackEntity{st})
	if resp := svc.checkNoActiveDeployingStack(); resp != nil {
		t.Errorf("LocalDevnet deploying should not block, got %+v", resp)
	}
}

func TestCheckNoActiveDeployingStack_DeployedAllowed(t *testing.T) {
	// Deployed = L1 contracts done; safe to start another deployment.
	st := makeDeployingStack(t, entities.DeploymentNetworkTestnet, "aws", entities.StackStatusDeployed)
	svc := svcWithStacks(t, []*entities.StackEntity{st})
	if resp := svc.checkNoActiveDeployingStack(); resp != nil {
		t.Errorf("Deployed stack should not block new deployment, got %+v", resp)
	}
}

func TestCheckNoActiveDeployingStack_TerminalStatusesAllowed(t *testing.T) {
	for _, status := range []entities.StackStatus{
		entities.StackStatusTerminated,
		entities.StackStatusStopped,
		entities.StackStatusFailedToDeploy,
		entities.StackStatusFailedToTerminate,
	} {
		t.Run(string(status), func(t *testing.T) {
			st := makeDeployingStack(t, entities.DeploymentNetworkTestnet, "aws", status)
			svc := svcWithStacks(t, []*entities.StackEntity{st})
			if resp := svc.checkNoActiveDeployingStack(); resp != nil {
				t.Errorf("status %q should not block, got %+v", status, resp)
			}
		})
	}
}

func TestCheckNoActiveDeployingStack_AWSDeployingBlocksNewLocalDeploy(t *testing.T) {
	// Core regression: AWS Deploying must block new local (Testnet) deployment.
	awsStack := makeDeployingStack(t, entities.DeploymentNetworkTestnet, "aws", entities.StackStatusDeploying)
	svc := svcWithStacks(t, []*entities.StackEntity{awsStack})
	resp := svc.checkNoActiveDeployingStack()
	if resp == nil {
		t.Fatal("expected 409 when AWS stack is deploying, got nil")
	}
	if resp.Status != http.StatusConflict {
		t.Errorf("expected HTTP 409, got %d", resp.Status)
	}
}

func TestCheckNoActiveDeployingStack_AWSDeployingBlocksNewAWSDeploy(t *testing.T) {
	// AWS+Testnet must block another AWS+Testnet deployment.
	awsStack := makeDeployingStack(t, entities.DeploymentNetworkTestnet, "aws", entities.StackStatusDeploying)
	svc := svcWithStacks(t, []*entities.StackEntity{awsStack})
	resp := svc.checkNoActiveDeployingStack()
	if resp == nil {
		t.Fatal("expected 409 when another AWS stack is deploying, got nil")
	}
	if resp.Status != http.StatusConflict {
		t.Errorf("expected HTTP 409, got %d", resp.Status)
	}
}

func TestCheckNoActiveDeployingStack_DBErrorAllows(t *testing.T) {
	logger.Init()
	// If the DB query fails, fail open (allow deployment) rather than blocking.
	svc := &ThanosStackDeploymentService{
		stackRepo: &allStacksRepo{err: assert_error("db down")},
	}
	if resp := svc.checkNoActiveDeployingStack(); resp != nil {
		t.Errorf("DB error should fail open (nil), got %+v", resp)
	}
}

// assert_error is a minimal error value for tests.
type assert_error string

func (e assert_error) Error() string { return string(e) }
