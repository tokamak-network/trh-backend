package thanos

import (
	"testing"

	"github.com/google/uuid"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/constants"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

// mockDeploymentRepo implements DeploymentRepository for tests.
type mockDeploymentRepo struct {
	deployments []*entities.DeploymentEntity
}

func (m *mockDeploymentRepo) CreateDeployment(d *entities.DeploymentEntity) error { return nil }
func (m *mockDeploymentRepo) GetDeploymentByStepAndStatus(stackID, step string, status entities.DeploymentStatus) (*entities.DeploymentEntity, error) {
	return nil, nil
}
func (m *mockDeploymentRepo) GetDeploymentsByStackID(stackId string) ([]*entities.DeploymentEntity, error) {
	return m.deployments, nil
}
func (m *mockDeploymentRepo) UpdateDeploymentStatus(deploymentId string, status entities.DeploymentRunStatus) error {
	return nil
}
func (m *mockDeploymentRepo) GetDeploymentByID(deploymentId string) (*entities.DeploymentEntity, error) {
	return nil, nil
}
func (m *mockDeploymentRepo) GetDeploymentStatus(deploymentId string) (entities.DeploymentRunStatus, error) {
	return entities.DeploymentRunStatusPending, nil
}
func (m *mockDeploymentRepo) GetDeploymentsByStackIDAndStatus(stackId string, status entities.DeploymentRunStatus) ([]*entities.DeploymentEntity, error) {
	return nil, nil
}
func (m *mockDeploymentRepo) UpdateStatusesByStackId(stackID string, status entities.DeploymentRunStatus) error {
	return nil
}

func newTestService(repo DeploymentRepository) *ThanosStackDeploymentService {
	return &ThanosStackDeploymentService{
		deploymentRepo: repo,
		name:           "test",
	}
}

func TestDeploymentService_LocalTestnet_SkipsAWS(t *testing.T) {
	stackId := uuid.New()
	svc := newTestService(&mockDeploymentRepo{deployments: []*entities.DeploymentEntity{}})

	config := &dtos.DeployThanosRequest{
		Network:        entities.DeploymentNetworkLocalTestnet,
		ChainName:      "TestChain",
		L1RpcUrl:       "https://sepolia.example.com",
		L1BeaconUrl:    "https://beacon.sepolia.example.com",
		KubeconfigPath: "/tmp/test.kubeconfig",
		InfraProvider:  "local",
	}

	deployments, err := svc.getThanosStackDeployments(stackId, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Main uses unified deploy-aws-infra step with InfraProvider field
	hasInfra := false
	for _, d := range deployments {
		if d.Step == constants.DeployInfraStep {
			hasInfra = true
		}
	}
	if !hasInfra {
		t.Error("infra step should be created for LocalTestnet")
	}
}

func TestDeploymentService_Testnet_UsesAWS(t *testing.T) {
	stackId := uuid.New()
	svc := newTestService(&mockDeploymentRepo{deployments: []*entities.DeploymentEntity{}})

	config := &dtos.DeployThanosRequest{
		Network:            entities.DeploymentNetworkTestnet,
		ChainName:          "TestChain",
		L1RpcUrl:           "https://sepolia.example.com",
		L1BeaconUrl:        "https://beacon.sepolia.example.com",
		AwsAccessKey:       "AKIAIOSFODNN7EXAMPLE",
		AwsSecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		AwsRegion:          "ap-northeast-2",
	}

	deployments, err := svc.getThanosStackDeployments(stackId, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, d := range deployments {
		if d.Step == constants.DeployLocalInfraStep {
			t.Errorf("deploy-local-infra step should NOT be created for Testnet, but found it")
		}
	}

	hasAWSInfra := false
	for _, d := range deployments {
		if d.Step == constants.DeployInfraStep {
			hasAWSInfra = true
		}
	}
	if !hasAWSInfra {
		t.Error("deploy-aws-infra step should be created for Testnet, but was not found")
	}
}
