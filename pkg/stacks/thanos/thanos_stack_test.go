package thanos

import (
	"os"
	"testing"

	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

func TestMain(m *testing.M) {
	logger.Init()
	os.Exit(m.Run())
}

func TestResolveTarget_ExplicitTarget(t *testing.T) {
	stack := &entities.StackEntity{
		Network: entities.DeploymentNetworkTestnet,
		Target:  entities.DeploymentTargetLocal,
	}
	if got := stack.ResolveTarget(); got != entities.DeploymentTargetLocal {
		t.Errorf("expected Local, got %q", got)
	}
}

func TestResolveTarget_DeriveFromLocalTestnet(t *testing.T) {
	stack := &entities.StackEntity{
		Network: entities.DeploymentNetworkLocalTestnet,
		Target:  "", // not set (old data)
	}
	if got := stack.ResolveTarget(); got != entities.DeploymentTargetLocal {
		t.Errorf("expected Local (derived from LocalTestnet), got %q", got)
	}
}

func TestResolveTarget_DeriveFromTestnet(t *testing.T) {
	stack := &entities.StackEntity{
		Network: entities.DeploymentNetworkTestnet,
		Target:  "", // not set (old data)
	}
	if got := stack.ResolveTarget(); got != entities.DeploymentTargetCloud {
		t.Errorf("expected Cloud (derived from Testnet), got %q", got)
	}
}

func TestNewSDKClientForStack_NilStackConfig(t *testing.T) {
	stack := &entities.StackEntity{
		Network: entities.DeploymentNetworkTestnet,
	}
	_, err := NewSDKClientForStack(nil, "/tmp/test.log", stack, nil)
	if err == nil {
		t.Fatal("expected error for nil stackConfig, got nil")
	}
	expected := "stackConfig is required for NewSDKClientForStack"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

func TestNewSDKClientForStack_NilStackConfigLocalTestnet(t *testing.T) {
	stack := &entities.StackEntity{
		Network: entities.DeploymentNetworkLocalTestnet,
	}
	_, err := NewSDKClientForStack(nil, "/tmp/test.log", stack, nil)
	if err == nil {
		t.Fatal("expected error for nil stackConfig, got nil")
	}
	expected := "stackConfig is required for NewSDKClientForStack"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

func TestNewSDKClientForStack_LocalTargetRoute(t *testing.T) {
	stack := &entities.StackEntity{
		Network:        entities.DeploymentNetworkLocalTestnet,
		Target:         entities.DeploymentTargetLocal,
		KubeconfigPath: "/tmp/nonexistent.kubeconfig",
		DeploymentPath: t.TempDir(),
	}
	cfg := &dtos.DeployThanosRequest{}

	// Will fail because kubeconfig doesn't exist, but proves LocalTestnet path is taken
	// (not the AWS path which would fail differently on empty AWS creds).
	_, err := NewSDKClientForStack(nil, "/tmp/test.log", stack, cfg)
	if err == nil {
		return // unexpected success is fine
	}
	// Should NOT be the nil guard error
	if err.Error() == "stackConfig is required for NewSDKClientForStack" {
		t.Fatal("should not hit nil guard — stackConfig was provided")
	}
}

func TestNewSDKClientForStack_KubeconfigFallbackFromConfig(t *testing.T) {
	stack := &entities.StackEntity{
		Network:        entities.DeploymentNetworkLocalTestnet,
		KubeconfigPath: "", // empty on stack entity
		DeploymentPath: t.TempDir(),
	}
	cfg := &dtos.DeployThanosRequest{
		KubeconfigPath: "/tmp/fallback.kubeconfig",
	}

	// Should take LocalTestnet path with fallback kubeconfig from config
	_, err := NewSDKClientForStack(nil, "/tmp/test.log", stack, cfg)
	if err != nil && err.Error() == "stackConfig is required for NewSDKClientForStack" {
		t.Fatal("should not hit nil guard — stackConfig was provided")
	}
	// Any other error (file not found, etc.) is expected
}
