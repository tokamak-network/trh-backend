package dtos

import (
	"os"
	"testing"

	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

func TestMain(m *testing.M) {
	logger.Init()
	m.Run()
}

func TestValidation_LocalTestnet_NeedKubeconfig(t *testing.T) {
	req := &DeployThanosRequest{
		Network:                  entities.DeploymentNetworkLocalTestnet,
		L1RpcUrl:                 "https://sepolia.example.com",
		L1BeaconUrl:              "https://beacon.sepolia.example.com",
		L2BlockTime:              2,
		BatchSubmissionFrequency: 120,
		OutputRootFrequency:      120,
		ChallengePeriod:          120,
		AdminAccount:             "0x0000000000000000000000000000000000000001",
		SequencerAccount:         "0x0000000000000000000000000000000000000002",
		BatcherAccount:           "0x0000000000000000000000000000000000000003",
		ProposerAccount:          "0x0000000000000000000000000000000000000004",
		ChainName:                "TestChain",
		// KubeconfigPath intentionally omitted
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("expected error when kubeconfigPath is missing for LocalTestnet, got nil")
	}
	if err.Error() != "kubeconfigPath is required for local testnet deployment" {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

func TestValidation_LocalTestnet_NoAWSRequired(t *testing.T) {
	// Create a real (empty) kubeconfig so the file-existence check passes.
	// This ensures we reach the AWS validation block — proving it is skipped,
	// not just that we errored out earlier.
	tmpFile, err := os.CreateTemp("", "trh-test-*.kubeconfig")
	if err != nil {
		t.Fatalf("failed to create temp kubeconfig: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp kubeconfig: %v", err)
	}

	req := &DeployThanosRequest{
		Network:                  entities.DeploymentNetworkLocalTestnet,
		L1RpcUrl:                 "https://sepolia.example.com",
		L1BeaconUrl:              "https://beacon.sepolia.example.com",
		L2BlockTime:              2,
		BatchSubmissionFrequency: 120,
		OutputRootFrequency:      120,
		ChallengePeriod:          120,
		AdminAccount:             "0x0000000000000000000000000000000000000001",
		SequencerAccount:         "0x0000000000000000000000000000000000000002",
		BatcherAccount:           "0x0000000000000000000000000000000000000003",
		ProposerAccount:          "0x0000000000000000000000000000000000000004",
		ChainName:                "TestChain",
		KubeconfigPath:           tmpFile.Name(),
		// AWS fields intentionally omitted
	}

	err = req.Validate()
	// AWS validation is skipped for LocalTestnet so the error (if any) should
	// NOT be about AWS credentials. We only expect chain/RPC validation errors here.
	awsErrors := []string{"invalid awsAccessKey", "invalid awsSecretKey", "invalid awsRegion"}
	if err != nil {
		for _, awsErr := range awsErrors {
			if err.Error() == awsErr {
				t.Fatalf("AWS validation should be skipped for LocalTestnet, got: %s", err.Error())
			}
		}
	}
}

func TestValidation_MissingAccounts_WithoutSeedPhrase(t *testing.T) {
	req := &DeployThanosRequest{
		Network:                  entities.DeploymentNetworkLocalTestnet,
		L1RpcUrl:                 "https://sepolia.example.com",
		L1BeaconUrl:              "https://beacon.sepolia.example.com",
		L2BlockTime:              2,
		BatchSubmissionFrequency: 120,
		OutputRootFrequency:      120,
		ChallengePeriod:          120,
		ChainName:                "TestChain",
		KubeconfigPath:           "/some/path",
		// Neither operator accounts nor SeedPhrase provided — must error
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("expected error when operator accounts and seedPhrase are both absent")
	}
	expected := "adminAccount, sequencerAccount, batcherAccount, and proposerAccount are required when seedPhrase is not provided"
	if err.Error() != expected {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}

func TestValidation_SeedPhrase_AllowsMissingAccounts(t *testing.T) {
	req := &DeployThanosRequest{
		Network:                  entities.DeploymentNetworkLocalTestnet,
		L1RpcUrl:                 "https://sepolia.example.com",
		L1BeaconUrl:              "https://beacon.sepolia.example.com",
		L2BlockTime:              2,
		BatchSubmissionFrequency: 120,
		OutputRootFrequency:      120,
		ChallengePeriod:          120,
		ChainName:                "TestChain",
		KubeconfigPath:           "/some/path",
		SeedPhrase:               "test test test test test test test test test test test junk",
		// Operator accounts intentionally omitted — should not error on missing accounts
	}

	err := req.Validate()
	// The account-missing check must pass; other errors (RPC, chain) are acceptable.
	if err != nil {
		accountErr := "adminAccount, sequencerAccount, batcherAccount, and proposerAccount are required when seedPhrase is not provided"
		if err.Error() == accountErr {
			t.Fatalf("should not require accounts when seedPhrase is provided: %s", err.Error())
		}
	}
}

func TestValidation_SeedPhrase_RejectsConflictWithAccounts(t *testing.T) {
	req := &DeployThanosRequest{
		Network:                  entities.DeploymentNetworkLocalTestnet,
		L1RpcUrl:                 "https://sepolia.example.com",
		L1BeaconUrl:              "https://beacon.sepolia.example.com",
		L2BlockTime:              2,
		BatchSubmissionFrequency: 120,
		OutputRootFrequency:      120,
		ChallengePeriod:          120,
		ChainName:                "TestChain",
		KubeconfigPath:           "/some/path",
		SeedPhrase:               "test test test test test test test test test test test junk",
		AdminAccount:             "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266", // conflict
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("expected error when both seedPhrase and operator accounts are provided")
	}
	expected := "provide either seedPhrase or operator accounts, not both"
	if err.Error() != expected {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}

func TestValidation_Testnet_RequiresAWS(t *testing.T) {
	req := &DeployThanosRequest{
		Network:                  entities.DeploymentNetworkTestnet,
		L1RpcUrl:                 "https://sepolia.example.com",
		L1BeaconUrl:              "https://beacon.sepolia.example.com",
		L2BlockTime:              2,
		BatchSubmissionFrequency: 120,
		OutputRootFrequency:      120,
		ChallengePeriod:          120,
		AdminAccount:             "0x0000000000000000000000000000000000000001",
		SequencerAccount:         "0x0000000000000000000000000000000000000002",
		BatcherAccount:           "0x0000000000000000000000000000000000000003",
		ProposerAccount:          "0x0000000000000000000000000000000000000004",
		ChainName:                "TestChain",
		// AWS fields intentionally omitted → should fail validation
	}

	err := req.Validate()
	if err == nil {
		t.Fatal("expected error when AWS credentials are missing for Testnet")
	}
}
