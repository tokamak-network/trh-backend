package thanos

import (
	"context"
	"testing"

	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/constants"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

// TestGetThanosStackDeployments_LocalTestnet_IncludesL1Contracts verifies that
// local+Testnet creates both deploy-l1-contracts and deploy-local-infra steps.
//
// TDD RED: Current code skips deploy-l1-contracts for all local providers.
// TDD GREEN: After fixing to check Network instead of InfraProvider.
func TestGetThanosStackDeployments_LocalTestnet_IncludesL1Contracts(t *testing.T) {
	logger.Init()

	stackRepo := &captureStackRepo{}
	service := &ThanosStackDeploymentService{
		name:           "Thanos",
		deploymentRepo: &emptyDeploymentRepo{},
		stackRepo:      stackRepo,
		taskManager:    &noopTaskManager{},
	}

	resp, err := service.CreateThanosStackFromPreset(context.Background(), dtos.PresetDeployRequest{
		PresetID:      "general",
		ChainName:     "TestChain",
		Network:       entities.DeploymentNetworkTestnet,
		SeedPhrase:    drbTestMnemonic,
		InfraProvider: "local",
		L1RpcUrl:      "http://rpc.sepolia.example",
		L1BeaconUrl:   "http://beacon.sepolia.example",
		FeeToken:      "TON",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.Status != 200 {
		t.Fatalf("unexpected response: %#v", resp)
	}

	hasL1Contracts := false
	hasLocalInfra := false
	for _, d := range stackRepo.deployments {
		if d.Step == constants.DeployL1ContractsStep {
			hasL1Contracts = true
		}
		if d.Step == constants.DeployLocalInfraStep {
			hasLocalInfra = true
		}
	}

	if !hasL1Contracts {
		t.Errorf("local+Testnet must include deploy-l1-contracts step; got steps: %v", deploymentStepNames(stackRepo.deployments))
	}
	if !hasLocalInfra {
		t.Errorf("local+Testnet must include deploy-local-infra step; got steps: %v", deploymentStepNames(stackRepo.deployments))
	}
}

// TestGetThanosStackDeployments_LocalDevnet_SkipsL1Contracts verifies that
// local+LocalDevnet does NOT create a deploy-l1-contracts step because
// SDK.Deploy() handles L1+L2 together for local_devnet.
func TestGetThanosStackDeployments_LocalDevnet_SkipsL1Contracts(t *testing.T) {
	logger.Init()

	stackRepo := &captureStackRepo{}
	service := &ThanosStackDeploymentService{
		name:           "Thanos",
		deploymentRepo: &emptyDeploymentRepo{},
		stackRepo:      stackRepo,
		taskManager:    &noopTaskManager{},
	}

	resp, err := service.CreateThanosStackFromPreset(context.Background(), dtos.PresetDeployRequest{
		PresetID:      "general",
		ChainName:     "TestChain",
		Network:       entities.DeploymentNetworkLocalDevnet,
		SeedPhrase:    drbTestMnemonic,
		InfraProvider: "local",
		L1RpcUrl:      "http://localhost:8545",
		L1BeaconUrl:   "http://localhost:5052",
		FeeToken:      "TON",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.Status != 200 {
		t.Fatalf("unexpected response: %#v", resp)
	}

	for _, d := range stackRepo.deployments {
		if d.Step == constants.DeployL1ContractsStep {
			t.Errorf("local+LocalDevnet must NOT include deploy-l1-contracts step; got steps: %v", deploymentStepNames(stackRepo.deployments))
			break
		}
	}
}

func deploymentStepNames(deployments []*entities.DeploymentEntity) []string {
	names := make([]string, len(deployments))
	for i, d := range deployments {
		names[i] = d.Step
	}
	return names
}
