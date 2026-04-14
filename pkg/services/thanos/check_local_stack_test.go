package thanos

// Internal package test — access to unexported checkNoActiveLocalStack.

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

// allStacksRepo is a StackRepository stub where GetAllStacks returns a preset list.
type allStacksRepo struct {
	stacks []*entities.StackEntity
	err    error
}

func (r *allStacksRepo) GetAllStacks() ([]*entities.StackEntity, error) {
	return r.stacks, r.err
}

// Remaining interface methods — unused in these tests.
func (r *allStacksRepo) CreateStackByTx(*entities.StackEntity, []*entities.DeploymentEntity, []*entities.IntegrationEntity) error {
	return nil
}
func (r *allStacksRepo) UpdateStatus(string, entities.StackStatus, string) error { return nil }
func (r *allStacksRepo) GetStackByID(_ string) (*entities.StackEntity, error)    { return nil, nil }
func (r *allStacksRepo) GetStackStatus(string) (entities.StackStatus, error)     { return "", nil }
func (r *allStacksRepo) UpdateMetadata(string, *entities.StackMetadata) error    { return nil }
func (r *allStacksRepo) UpdateConfig(string, []byte) error                       { return nil }

// makeLocalStack builds a minimal StackEntity for local infrastructure.
func makeLocalStack(t *testing.T, status entities.StackStatus) *entities.StackEntity {
	t.Helper()
	cfg := dtos.DeployThanosRequest{InfraProvider: "local"}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	return &entities.StackEntity{
		ID:     uuid.New(),
		Status: status,
		Config: raw,
	}
}

// svcWithStacks constructs a minimal ThanosStackDeploymentService with the given stacks.
func svcWithStacks(t *testing.T, stacks []*entities.StackEntity) *ThanosStackDeploymentService {
	t.Helper()
	return &ThanosStackDeploymentService{
		stackRepo: &allStacksRepo{stacks: stacks},
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests
// ──────────────────────────────────────────────────────────────────────────────

func TestCheckNoActiveLocalStack_NoStacks(t *testing.T) {
	svc := svcWithStacks(t, nil)
	if resp := svc.checkNoActiveLocalStack(); resp != nil {
		t.Errorf("expected nil response when no stacks, got %v", resp)
	}
}

func TestCheckNoActiveLocalStack_ActiveLocalBlocked(t *testing.T) {
	for _, status := range []entities.StackStatus{
		entities.StackStatusPending,
		entities.StackStatusDeploying,
		entities.StackStatusDeployed,
		entities.StackStatusUpdating,
	} {
		t.Run(string(status), func(t *testing.T) {
			svc := svcWithStacks(t, []*entities.StackEntity{makeLocalStack(t, status)})
			resp := svc.checkNoActiveLocalStack()
			if resp == nil {
				t.Fatal("expected 409 response, got nil")
			}
			if resp.Status != http.StatusConflict {
				t.Errorf("expected HTTP 409, got %d", resp.Status)
			}
		})
	}
}

func TestCheckNoActiveLocalStack_TerminalStatusAllowed(t *testing.T) {
	for _, status := range []entities.StackStatus{
		entities.StackStatusTerminated,
		entities.StackStatusStopped,
		entities.StackStatusFailedToDeploy,
		entities.StackStatusFailedToTerminate,
	} {
		t.Run(string(status), func(t *testing.T) {
			svc := svcWithStacks(t, []*entities.StackEntity{makeLocalStack(t, status)})
			if resp := svc.checkNoActiveLocalStack(); resp != nil {
				t.Errorf("status %q should not block new deploy, got %+v", status, resp)
			}
		})
	}
}

func TestCheckNoActiveLocalStack_NonLocalActiveIgnored(t *testing.T) {
	// An EC2/K8s stack in Deployed state should not block a local deploy.
	cfg := dtos.DeployThanosRequest{InfraProvider: "aws"}
	raw, _ := json.Marshal(cfg)
	awsStack := &entities.StackEntity{
		ID:     uuid.New(),
		Status: entities.StackStatusDeployed,
		Config: raw,
	}
	svc := svcWithStacks(t, []*entities.StackEntity{awsStack})
	if resp := svc.checkNoActiveLocalStack(); resp != nil {
		t.Errorf("non-local active stack should not block local deploy, got %+v", resp)
	}
}
