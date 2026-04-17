package thanos

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/constants"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

const drbTestMnemonic = "test test test test test test test test test test test junk"

type captureStackRepo struct {
	stack        *entities.StackEntity
	deployments  []*entities.DeploymentEntity
	integrations []*entities.IntegrationEntity
}

func (r *captureStackRepo) CreateStackByTx(
	stack *entities.StackEntity,
	deployments []*entities.DeploymentEntity,
	integrations []*entities.IntegrationEntity,
) error {
	r.stack = stack
	r.deployments = deployments
	r.integrations = integrations
	return nil
}

func (r *captureStackRepo) UpdateStatus(string, entities.StackStatus, string) error {
	return nil
}

func (r *captureStackRepo) GetStackByID(string) (*entities.StackEntity, error) {
	return r.stack, nil
}

func (r *captureStackRepo) GetAllStacks() ([]*entities.StackEntity, error) {
	return nil, nil
}

func (r *captureStackRepo) GetStackStatus(string) (entities.StackStatus, error) {
	return "", nil
}

func (r *captureStackRepo) UpdateMetadata(string, *entities.StackMetadata) error {
	return nil
}

func (r *captureStackRepo) UpdateConfig(string, []byte) error {
	return nil
}

type emptyDeploymentRepo struct{}

func (r *emptyDeploymentRepo) CreateDeployment(*entities.DeploymentEntity) error {
	return nil
}

func (r *emptyDeploymentRepo) GetDeploymentByStepAndStatus(string, string, entities.DeploymentStatus) (*entities.DeploymentEntity, error) {
	return nil, nil
}

func (r *emptyDeploymentRepo) GetDeploymentsByStackID(string) ([]*entities.DeploymentEntity, error) {
	return nil, nil
}

func (r *emptyDeploymentRepo) UpdateDeploymentStatus(string, entities.DeploymentRunStatus) error {
	return nil
}

func (r *emptyDeploymentRepo) GetDeploymentByID(string) (*entities.DeploymentEntity, error) {
	return nil, nil
}

func (r *emptyDeploymentRepo) GetDeploymentStatus(string) (entities.DeploymentRunStatus, error) {
	return "", nil
}

func (r *emptyDeploymentRepo) GetDeploymentsByStackIDAndStatus(string, entities.DeploymentRunStatus) ([]*entities.DeploymentEntity, error) {
	return nil, nil
}

func (r *emptyDeploymentRepo) UpdateStatusesByStackId(string, entities.DeploymentRunStatus) error {
	return nil
}

type noopTaskManager struct{}

func (m *noopTaskManager) Start() {}

func (m *noopTaskManager) AddTask(string, entities.Task) {}

func (m *noopTaskManager) AddTaskWithProgress(string, func(context.Context, func(string, float64))) {}

func (m *noopTaskManager) SetTaskResult(string, any) {}

func (m *noopTaskManager) StopTask(string) {}

func (m *noopTaskManager) IsTaskRunning(string) bool { return false }

func (m *noopTaskManager) Stop() {}

func TestCreateThanosStackFromPreset_PropagatesSeedPhraseToDeploymentConfigs(t *testing.T) {
	logger.Init()

	stackRepo := &captureStackRepo{}
	service := &ThanosStackDeploymentService{
		name:           "Thanos",
		deploymentRepo: &emptyDeploymentRepo{},
		stackRepo:      stackRepo,
		taskManager:    &noopTaskManager{},
	}

	response, err := service.CreateThanosStackFromPreset(context.Background(), dtos.PresetDeployRequest{
		PresetID:      "gaming",
		ChainName:     "DRBChain",
		Network:       entities.DeploymentNetworkTestnet,
		SeedPhrase:    drbTestMnemonic,
		InfraProvider: "local",
		L1RpcUrl:      "http://localhost:8545",
		L1BeaconUrl:   "http://localhost:5052",
		FeeToken:      "TON",
	})
	if err != nil {
		t.Fatalf("CreateThanosStackFromPreset returned error: %v", err)
	}
	if response == nil || response.Status != 200 {
		t.Fatalf("unexpected response: %#v", response)
	}

	var stackConfig dtos.DeployThanosRequest
	if err := json.Unmarshal(stackRepo.stack.Config, &stackConfig); err != nil {
		t.Fatalf("failed to unmarshal stack config: %v", err)
	}
	if stackConfig.SeedPhrase != drbTestMnemonic {
		t.Fatalf("stack config seedPhrase = %q, want %q", stackConfig.SeedPhrase, drbTestMnemonic)
	}

	var deployContractsConfig dtos.DeployL1ContractsRequest
	var deployInfraConfig dtos.DeployThanosAWSInfraRequest
	for _, deployment := range stackRepo.deployments {
		switch deployment.Step {
		case constants.DeployL1ContractsStep:
			if err := json.Unmarshal(deployment.Config, &deployContractsConfig); err != nil {
				t.Fatalf("failed to unmarshal deploy contracts config: %v", err)
			}
		case constants.DeployInfraStep:
			if err := json.Unmarshal(deployment.Config, &deployInfraConfig); err != nil {
				t.Fatalf("failed to unmarshal deploy infra config: %v", err)
			}
		}
	}

	if deployContractsConfig.SeedPhrase != drbTestMnemonic {
		t.Fatalf("deploy contracts seedPhrase = %q, want %q", deployContractsConfig.SeedPhrase, drbTestMnemonic)
	}
	if deployInfraConfig.SeedPhrase != drbTestMnemonic {
		t.Fatalf("deploy infra seedPhrase = %q, want %q", deployInfraConfig.SeedPhrase, drbTestMnemonic)
	}
	if stackRepo.stack.ID == uuid.Nil {
		t.Fatal("expected stack to be created with a non-nil ID")
	}
}
