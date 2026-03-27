package thanos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

func TestMain(m *testing.M) {
	logger.Init()
	os.Exit(m.Run())
}

// knownMnemonic is the Hardhat/Anvil test mnemonic (Hardhat account 0–3).
const knownMnemonic = "test test test test test test test test test test test junk"

// Canonical Hardhat addresses (verified at https://iancoleman.io/bip39/).
const (
	wantAdmin     = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
	wantSequencer = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
	wantBatcher   = "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"
	wantProposer  = "0x90F79bf6EB2c4f870365E785982E1f101E93b906"
)

func newKeyTestService() *ThanosStackDeploymentService {
	return newTestService(&mockDeploymentRepo{deployments: []*entities.DeploymentEntity{}})
}

// newRPCMockServer returns a minimal JSON-RPC HTTP server for testing.
// It responds to eth_blockNumber and eth_getBalance requests.
func newRPCMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	type rpcMsg struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
	}
	respond := func(w http.ResponseWriter, id json.RawMessage, result string) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":"%s"}`, id, result)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		// Handle JSON-RPC batch (array) or single request.
		if len(body) > 0 && body[0] == '[' {
			var msgs []rpcMsg
			if err := json.Unmarshal(body, &msgs); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("["))
			for i, msg := range msgs {
				if i > 0 {
					w.Write([]byte(","))
				}
				switch msg.Method {
				case "eth_blockNumber":
					fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":"0x1"}`, msg.ID)
				default:
					// 0xde0b6b3a7640000 = 10^18 wei = 1.0 ETH
					fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":"0xde0b6b3a7640000"}`, msg.ID)
				}
			}
			w.Write([]byte("]"))
			return
		}

		var msg rpcMsg
		if err := json.Unmarshal(body, &msg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		switch msg.Method {
		case "eth_blockNumber":
			respond(w, msg.ID, "0x1")
		default:
			// 0xde0b6b3a7640000 = 10^18 wei = 1.0 ETH
			respond(w, msg.ID, "0xde0b6b3a7640000")
		}
	}))
}

// ---------------------------------------------------------------------------
// DeriveKeys tests
// ---------------------------------------------------------------------------

// TestDeriveKeys_NoRPC verifies key derivation without an L1 RPC URL (balance check skipped).
func TestDeriveKeys_NoRPC(t *testing.T) {
	svc := newKeyTestService()
	resp, err := svc.DeriveKeys(context.Background(), &dtos.DeriveKeysRequest{
		SeedPhrase: knownMnemonic,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Status, resp.Message)
	}

	data, ok := resp.Data.(*dtos.DeriveKeysResponse)
	if !ok {
		t.Fatalf("response data is not *DeriveKeysResponse")
	}

	// Verify all four operator addresses match canonical Hardhat values.
	for _, tc := range []struct{ got, want, role string }{
		{data.Admin.Address, wantAdmin, "admin"},
		{data.Sequencer.Address, wantSequencer, "sequencer"},
		{data.Batcher.Address, wantBatcher, "batcher"},
		{data.Proposer.Address, wantProposer, "proposer"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s address = %q, want %q", tc.role, tc.got, tc.want)
		}
	}
	// No balance fields expected when L1RpcUrl is empty.
	if data.Admin.BalanceEth != "" || data.Admin.BalanceError != "" {
		t.Errorf("expected no balance fields when L1RpcUrl is empty")
	}
}

// TestDeriveKeys_InvalidMnemonic verifies a 400 response for bad mnemonics.
func TestDeriveKeys_InvalidMnemonic(t *testing.T) {
	svc := newKeyTestService()
	resp, err := svc.DeriveKeys(context.Background(), &dtos.DeriveKeysRequest{
		SeedPhrase: "not a valid mnemonic phrase at all",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.Status)
	}
}

// TestDeriveKeys_WithPassphrase verifies that a BIP39 passphrase produces different keys.
func TestDeriveKeys_WithPassphrase(t *testing.T) {
	svc := newKeyTestService()

	resp1, _ := svc.DeriveKeys(context.Background(), &dtos.DeriveKeysRequest{SeedPhrase: knownMnemonic})
	resp2, _ := svc.DeriveKeys(context.Background(), &dtos.DeriveKeysRequest{SeedPhrase: knownMnemonic, BIP39Passphrase: "secret"})

	data1 := resp1.Data.(*dtos.DeriveKeysResponse)
	data2 := resp2.Data.(*dtos.DeriveKeysResponse)

	if data1.Admin.Address == data2.Admin.Address {
		t.Error("different passphrases must produce different addresses")
	}
}

// TestDeriveKeys_InvalidRPCURL verifies that an invalid L1RpcUrl returns 400 (SSRF guard).
func TestDeriveKeys_InvalidRPCURL(t *testing.T) {
	svc := newKeyTestService()
	resp, err := svc.DeriveKeys(context.Background(), &dtos.DeriveKeysRequest{
		SeedPhrase: knownMnemonic,
		L1RpcUrl:   "file:///etc/passwd",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid RPC URL, got %d: %s", resp.Status, resp.Message)
	}
}

// ---------------------------------------------------------------------------
// CheckAccountBalances tests
// ---------------------------------------------------------------------------

// TestCheckAccountBalances_InvalidRPCURL verifies that an invalid L1RpcUrl returns 400 (SSRF guard).
func TestCheckAccountBalances_InvalidRPCURL(t *testing.T) {
	svc := newKeyTestService()
	resp, err := svc.CheckAccountBalances(context.Background(), &dtos.CheckAccountBalancesRequest{
		L1RpcUrl: "file:///etc/passwd",
		Addresses: dtos.OperatorAddresses{
			Admin:     wantAdmin,
			Sequencer: wantSequencer,
			Batcher:   wantBatcher,
			Proposer:  wantProposer,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid RPC URL, got %d: %s", resp.Status, resp.Message)
	}
}

// TestCheckAccountBalances_InvalidAddress verifies a 400 response for non-hex addresses.
func TestCheckAccountBalances_InvalidAddress(t *testing.T) {
	svc := newKeyTestService()
	resp, err := svc.CheckAccountBalances(context.Background(), &dtos.CheckAccountBalancesRequest{
		L1RpcUrl: "http://localhost:8545",
		Addresses: dtos.OperatorAddresses{
			Admin:     "not-an-address",
			Sequencer: wantSequencer,
			Batcher:   wantBatcher,
			Proposer:  wantProposer,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid address, got %d: %s", resp.Status, resp.Message)
	}
}

// ---------------------------------------------------------------------------
// fetchBalance unit test (uses mock RPC server, bypasses IsValidL1RPC)
// ---------------------------------------------------------------------------

// TestFetchBalance_Success verifies that fetchBalance correctly retrieves and converts a balance.
func TestFetchBalance_Success(t *testing.T) {
	srv := newRPCMockServer(t)
	defer srv.Close()

	client, err := ethclient.DialContext(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("dial mock RPC: %v", err)
	}
	defer client.Close()

	bal, err := fetchBalance(context.Background(), client, wantAdmin)
	if err != nil {
		t.Fatalf("fetchBalance: %v", err)
	}
	// 0xde0b6b3a7640000 = 10^18 wei = exactly 1 ETH
	const wantBal = "1.000000000000000000"
	if bal != wantBal {
		t.Errorf("balance = %q, want %q", bal, wantBal)
	}
}

// TestFetchBalance_InvalidAddress verifies that fetchBalance rejects non-hex addresses.
func TestFetchBalance_InvalidAddress(t *testing.T) {
	srv := newRPCMockServer(t)
	defer srv.Close()

	client, err := ethclient.DialContext(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("dial mock RPC: %v", err)
	}
	defer client.Close()

	_, err = fetchBalance(context.Background(), client, "not-an-address")
	if err == nil {
		t.Error("expected error for invalid address, got nil")
	}
}

// ---------------------------------------------------------------------------
// weiToEth unit tests
// ---------------------------------------------------------------------------

func TestWeiToEth(t *testing.T) {
	cases := []struct {
		wei  string
		want string
	}{
		{"0", "0.000000000000000000"},
		{"1", "0.000000000000000001"},
		{"1000000000000000000", "1.000000000000000000"},          // 1 ETH
		{"500000000000000000", "0.500000000000000000"},           // 0.5 ETH
		{"1500000000000000000", "1.500000000000000000"},          // 1.5 ETH
		{"9223372036854775807", "9.223372036854775807"},          // MaxInt64 wei — big.Float precision boundary
		{"120000000000000000000000000", "120000000.000000000000000000"}, // ~total ETH supply in wei
	}
	for _, tc := range cases {
		w, ok := new(big.Int).SetString(tc.wei, 10)
		if !ok {
			t.Fatalf("invalid wei value: %s", tc.wei)
		}
		got := weiToEth(w)
		if got != tc.want {
			t.Errorf("weiToEth(%s) = %q, want %q", tc.wei, got, tc.want)
		}
	}
}
