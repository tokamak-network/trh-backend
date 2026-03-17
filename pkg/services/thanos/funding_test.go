package thanos

// Internal package test so we can inject private fields for isolation.

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"testing"

	"github.com/google/uuid"
	"github.com/ethereum/go-ethereum/common"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

// ── stubs ──────────────────────────────────────────────────────────────────

// stubEthClient returns pre-configured balances keyed by address hex string.
type stubEthClient struct {
	balances map[string]*big.Int
	dialErr  error
	queryErr error
}

func (c *stubEthClient) BalanceAt(_ context.Context, addr common.Address, _ *big.Int) (*big.Int, error) {
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	if b, ok := c.balances[addr.Hex()]; ok {
		return b, nil
	}
	return big.NewInt(0), nil
}
func (c *stubEthClient) Close() {}

// stubStackRepo satisfies StackRepository for tests that only use GetStackByID.
type stubStackRepo struct {
	stack *entities.StackEntity
	err   error
}

func (r *stubStackRepo) GetStackByID(_ string) (*entities.StackEntity, error) {
	return r.stack, r.err
}

// unused interface methods
func (r *stubStackRepo) CreateStackByTx(*entities.StackEntity, []*entities.DeploymentEntity, []*entities.IntegrationEntity) error {
	return nil
}
func (r *stubStackRepo) UpdateStatus(string, entities.StackStatus, string) error { return nil }
func (r *stubStackRepo) GetAllStacks() ([]*entities.StackEntity, error)          { return nil, nil }
func (r *stubStackRepo) GetStackStatus(string) (entities.StackStatus, error) {
	return "", nil
}
func (r *stubStackRepo) UpdateMetadata(string, *entities.StackMetadata) error { return nil }
func (r *stubStackRepo) UpdateConfig(string, []byte) error                    { return nil }

// ── helpers ────────────────────────────────────────────────────────────────

type account struct {
	role    string
	address string
	balance string // wei string
}

// validAddresses returns four dummy but non-empty ETH addresses.
var validAddresses = map[string]string{
	"admin":     "0x1111111111111111111111111111111111111111",
	"sequencer": "0x2222222222222222222222222222222222222222",
	"batcher":   "0x3333333333333333333333333333333333333333",
	"proposer":  "0x4444444444444444444444444444444444444444",
}

func makeStackConfig(t *testing.T, network entities.DeploymentNetwork, overrides map[string]string) *entities.StackEntity {
	t.Helper()
	cfg := dtos.DeployThanosRequest{
		Network:          network,
		L1RpcUrl:         "http://localhost:8545",
		AdminAccount:     validAddresses["admin"],
		SequencerAccount: validAddresses["sequencer"],
		BatcherAccount:   validAddresses["batcher"],
		ProposerAccount:  validAddresses["proposer"],
	}
	for role, addr := range overrides {
		switch role {
		case "admin":
			cfg.AdminAccount = addr
		case "sequencer":
			cfg.SequencerAccount = addr
		case "batcher":
			cfg.BatcherAccount = addr
		case "proposer":
			cfg.ProposerAccount = addr
		}
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	id := uuid.New()
	return &entities.StackEntity{ID: id, Network: network, Config: raw}
}

// eth address canonical form as go-ethereum returns it
func addrHex(addr string) string {
	return common.HexToAddress(addr).Hex()
}

func newServiceWithStubs(stackRepo StackRepository, ethClient EthBalanceClient) *ThanosStackDeploymentService {
	factory := func(_ context.Context, _ string) (EthBalanceClient, error) {
		return ethClient, nil
	}
	return &ThanosStackDeploymentService{
		stackRepo:        stackRepo,
		ethClientFactory: factory,
	}
}

// ── RequiredFundingByNetwork constant tests ────────────────────────────────

func TestRequiredFundingByNetwork_AllNetworksDefined(t *testing.T) {
	networks := []entities.DeploymentNetwork{
		entities.DeploymentNetworkTestnet,
		entities.DeploymentNetworkMainnet,
	}
	for _, network := range networks {
		reqs, ok := RequiredFundingByNetwork[network]
		if !ok {
			t.Errorf("RequiredFundingByNetwork missing entry for network %q", network)
			continue
		}
		for _, role := range []string{"admin", "sequencer", "batcher", "proposer"} {
			if _, ok := reqs[role]; !ok {
				t.Errorf("network %q: missing required funding for role %q", network, role)
			}
		}
	}
}

func TestRequiredFundingByNetwork_ValuesArePositiveIntegers(t *testing.T) {
	for network, roles := range RequiredFundingByNetwork {
		for role, weiStr := range roles {
			n := new(big.Int)
			if _, ok := n.SetString(weiStr, 10); !ok {
				t.Errorf("network %q role %q: %q is not a valid integer", network, role, weiStr)
				continue
			}
			if n.Sign() <= 0 {
				t.Errorf("network %q role %q: required balance must be positive, got %s", network, role, weiStr)
			}
		}
	}
}

func TestRequiredFundingByNetwork_MainnetHigherThanTestnet(t *testing.T) {
	testnet := RequiredFundingByNetwork[entities.DeploymentNetworkTestnet]
	mainnet := RequiredFundingByNetwork[entities.DeploymentNetworkMainnet]
	for _, role := range []string{"admin", "sequencer", "batcher", "proposer"} {
		tn, mn := new(big.Int), new(big.Int)
		tn.SetString(testnet[role], 10)
		mn.SetString(mainnet[role], 10)
		if mn.Cmp(tn) < 0 {
			t.Errorf("role %q: mainnet requirement (%s) < testnet (%s)", role, mainnet[role], testnet[role])
		}
	}
}

// ── GetFundingStatus unit tests ────────────────────────────────────────────

func TestGetFundingStatus_StackNotFound_ReturnsError(t *testing.T) {
	repo := &stubStackRepo{err: errors.New("record not found")}
	svc := newServiceWithStubs(repo, nil)

	_, err := svc.GetFundingStatus(context.Background(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error when stack not found, got nil")
	}
}

func TestGetFundingStatus_MissingAddress_Returns400Error(t *testing.T) {
	stack := makeStackConfig(t, entities.DeploymentNetworkTestnet, map[string]string{
		"admin": "", // intentionally empty
	})
	repo := &stubStackRepo{stack: stack}
	svc := newServiceWithStubs(repo, &stubEthClient{})

	_, err := svc.GetFundingStatus(context.Background(), stack.ID.String())
	if err == nil {
		t.Fatal("expected error for missing admin address, got nil")
	}
}

func TestGetFundingStatus_UnknownNetwork_ReturnsError(t *testing.T) {
	stack := makeStackConfig(t, entities.DeploymentNetworkLocalDevnet, map[string]string{})
	repo := &stubStackRepo{stack: stack}
	svc := newServiceWithStubs(repo, &stubEthClient{})

	_, err := svc.GetFundingStatus(context.Background(), stack.ID.String())
	if err == nil {
		t.Fatal("expected error for unsupported network, got nil")
	}
}

func TestGetFundingStatus_RPCDialError_ReturnsError(t *testing.T) {
	stack := makeStackConfig(t, entities.DeploymentNetworkTestnet, map[string]string{})
	repo := &stubStackRepo{stack: stack}

	dialErrFactory := func(_ context.Context, _ string) (EthBalanceClient, error) {
		return nil, errors.New("connection refused")
	}
	svc := &ThanosStackDeploymentService{
		stackRepo:        repo,
		ethClientFactory: dialErrFactory,
	}

	_, err := svc.GetFundingStatus(context.Background(), stack.ID.String())
	if err == nil {
		t.Fatal("expected error when RPC dial fails, got nil")
	}
}

func TestGetFundingStatus_BalanceQueryError_ReturnsError(t *testing.T) {
	stack := makeStackConfig(t, entities.DeploymentNetworkTestnet, map[string]string{})
	repo := &stubStackRepo{stack: stack}
	ethClient := &stubEthClient{queryErr: errors.New("network timeout")}
	svc := newServiceWithStubs(repo, ethClient)

	_, err := svc.GetFundingStatus(context.Background(), stack.ID.String())
	if err == nil {
		t.Fatal("expected error when balance query fails, got nil")
	}
}

func TestGetFundingStatus_AllAccountsFunded_AllFulfilledTrue(t *testing.T) {
	network := entities.DeploymentNetworkTestnet
	stack := makeStackConfig(t, network, map[string]string{})
	repo := &stubStackRepo{stack: stack}

	// Give each address more than the testnet requirement.
	aboveReq := new(big.Int).Mul(big.NewInt(10), new(big.Int).SetUint64(1e18)) // 10 ETH
	balances := map[string]*big.Int{
		addrHex(validAddresses["admin"]):     new(big.Int).Set(aboveReq),
		addrHex(validAddresses["sequencer"]): new(big.Int).Set(aboveReq),
		addrHex(validAddresses["batcher"]):   new(big.Int).Set(aboveReq),
		addrHex(validAddresses["proposer"]):  new(big.Int).Set(aboveReq),
	}
	svc := newServiceWithStubs(repo, &stubEthClient{balances: balances})

	resp, err := svc.GetFundingStatus(context.Background(), stack.ID.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.AllFulfilled {
		t.Error("expected AllFulfilled=true when all accounts have sufficient balance")
	}
	if len(resp.Accounts) != 4 {
		t.Errorf("expected 4 accounts, got %d", len(resp.Accounts))
	}
	for _, acc := range resp.Accounts {
		if !acc.Fulfilled {
			t.Errorf("role %q should be fulfilled but is not", acc.Role)
		}
	}
}

func TestGetFundingStatus_OneAccountUnderfunded_AllFulfilledFalse(t *testing.T) {
	network := entities.DeploymentNetworkTestnet
	stack := makeStackConfig(t, network, map[string]string{})
	repo := &stubStackRepo{stack: stack}

	aboveReq := new(big.Int).Mul(big.NewInt(10), new(big.Int).SetUint64(1e18))
	balances := map[string]*big.Int{
		addrHex(validAddresses["admin"]):     new(big.Int).Set(aboveReq),
		addrHex(validAddresses["sequencer"]): new(big.Int).Set(aboveReq),
		addrHex(validAddresses["batcher"]):   big.NewInt(1), // underfunded
		addrHex(validAddresses["proposer"]):  new(big.Int).Set(aboveReq),
	}
	svc := newServiceWithStubs(repo, &stubEthClient{balances: balances})

	resp, err := svc.GetFundingStatus(context.Background(), stack.ID.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AllFulfilled {
		t.Error("expected AllFulfilled=false when batcher is underfunded")
	}

	var batcher AccountFunding
	for _, acc := range resp.Accounts {
		if acc.Role == "batcher" {
			batcher = acc
			break
		}
	}
	if batcher.Fulfilled {
		t.Error("batcher should not be fulfilled")
	}
}

func TestGetFundingStatus_ResponseContainsStackIDAndNetwork(t *testing.T) {
	network := entities.DeploymentNetworkTestnet
	stack := makeStackConfig(t, network, map[string]string{})
	repo := &stubStackRepo{stack: stack}

	aboveReq := new(big.Int).Mul(big.NewInt(10), new(big.Int).SetUint64(1e18))
	balances := map[string]*big.Int{
		addrHex(validAddresses["admin"]):     new(big.Int).Set(aboveReq),
		addrHex(validAddresses["sequencer"]): new(big.Int).Set(aboveReq),
		addrHex(validAddresses["batcher"]):   new(big.Int).Set(aboveReq),
		addrHex(validAddresses["proposer"]):  new(big.Int).Set(aboveReq),
	}
	svc := newServiceWithStubs(repo, &stubEthClient{balances: balances})

	resp, err := svc.GetFundingStatus(context.Background(), stack.ID.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StackID != stack.ID.String() {
		t.Errorf("expected StackID %s, got %s", stack.ID, resp.StackID)
	}
	if resp.Network != string(network) {
		t.Errorf("expected Network %q, got %q", network, resp.Network)
	}
	if resp.CheckedAt == "" {
		t.Error("CheckedAt should not be empty")
	}
}

func TestGetFundingStatus_RoleOrderIsConsistent(t *testing.T) {
	stack := makeStackConfig(t, entities.DeploymentNetworkTestnet, map[string]string{})
	repo := &stubStackRepo{stack: stack}
	svc := newServiceWithStubs(repo, &stubEthClient{balances: map[string]*big.Int{}})

	resp, err := svc.GetFundingStatus(context.Background(), stack.ID.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedOrder := []string{"admin", "sequencer", "batcher", "proposer"}
	for i, role := range expectedOrder {
		if resp.Accounts[i].Role != role {
			t.Errorf("accounts[%d]: expected role %q, got %q", i, role, resp.Accounts[i].Role)
		}
	}
}
