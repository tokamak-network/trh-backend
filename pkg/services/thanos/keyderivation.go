package thanos

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/tokamak-network/trh-backend/internal/keyderivation"
	"github.com/tokamak-network/trh-backend/internal/logger"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
	trhSdkUtils "github.com/tokamak-network/trh-sdk/pkg/utils"
	"go.uber.org/zap"
)

// weiDecimals is the number of decimal places in one ETH (1 ETH = 10^18 wei).
const weiDecimals = 18

// rpcDialTimeout is the maximum time allowed to establish an RPC connection.
const rpcDialTimeout = 10 * time.Second

// rpcCallTimeout is the maximum time allowed for a single RPC call (e.g. eth_getBalance).
const rpcCallTimeout = 15 * time.Second

// DeriveKeys derives BIP44 operator private keys from a seed phrase.
// When req.L1RpcUrl is non-empty, each derived address is also queried for its ETH balance.
func (s *ThanosStackDeploymentService) DeriveKeys(ctx context.Context, req *dtos.DeriveKeysRequest) (*entities.Response, error) {
	admin, seq, batcher, proposer, err := keyderivation.DeriveOperatorKeys(req.SeedPhrase, req.BIP39Passphrase)
	if err != nil {
		logger.Error("key derivation failed", zap.Error(err))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "key derivation failed: " + err.Error(),
		}, nil
	}

	resp := &dtos.DeriveKeysResponse{
		Admin:     dtos.OperatorKeyInfo{PrivateKey: admin.PrivateKey, Address: admin.Address},
		Sequencer: dtos.OperatorKeyInfo{PrivateKey: seq.PrivateKey, Address: seq.Address},
		Batcher:   dtos.OperatorKeyInfo{PrivateKey: batcher.PrivateKey, Address: batcher.Address},
		Proposer:  dtos.OperatorKeyInfo{PrivateKey: proposer.PrivateKey, Address: proposer.Address},
	}

	if req.L1RpcUrl != "" {
		if !trhSdkUtils.IsValidL1RPC(req.L1RpcUrl) {
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: "invalid l1RpcUrl",
			}, nil
		}

		dialCtx, dialCancel := context.WithTimeout(ctx, rpcDialTimeout)
		client, dialErr := ethclient.DialContext(dialCtx, req.L1RpcUrl)
		dialCancel()
		if dialErr != nil {
			logger.Warn("could not connect to L1 RPC for balance check; balances will be omitted",
				zap.String("l1RpcUrl", req.L1RpcUrl),
				zap.Error(dialErr))
		} else {
			defer client.Close()
			fetchBalancesParallel(ctx, client, []*dtos.OperatorKeyInfo{
				&resp.Admin, &resp.Sequencer, &resp.Batcher, &resp.Proposer,
			})
		}
	}

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Keys derived successfully",
		Data:    resp,
	}, nil
}

// CheckAccountBalances fetches ETH balances on L1 for the given operator addresses.
func (s *ThanosStackDeploymentService) CheckAccountBalances(ctx context.Context, req *dtos.CheckAccountBalancesRequest) (*entities.Response, error) {
	// Validate addresses first — no network call required.
	for _, addr := range []string{req.Addresses.Admin, req.Addresses.Sequencer, req.Addresses.Batcher, req.Addresses.Proposer} {
		if !common.IsHexAddress(addr) {
			return &entities.Response{
				Status:  http.StatusBadRequest,
				Message: fmt.Sprintf("invalid Ethereum address: %s", addr),
			}, nil
		}
	}

	if !trhSdkUtils.IsValidL1RPC(req.L1RpcUrl) {
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "invalid l1RpcUrl",
		}, nil
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, rpcDialTimeout)
	client, err := ethclient.DialContext(dialCtx, req.L1RpcUrl)
	dialCancel()
	if err != nil {
		logger.Error("failed to connect to L1 RPC", zap.String("l1RpcUrl", req.L1RpcUrl), zap.Error(err))
		return &entities.Response{
			Status:  http.StatusBadRequest,
			Message: "failed to connect to L1 RPC: " + err.Error(),
		}, nil
	}
	defer client.Close()

	resp := &dtos.CheckAccountBalancesResponse{}

	type balanceEntry struct {
		addr   string
		result *dtos.AccountBalanceResult
	}
	entries := []balanceEntry{
		{req.Addresses.Admin, &resp.Admin},
		{req.Addresses.Sequencer, &resp.Sequencer},
		{req.Addresses.Batcher, &resp.Batcher},
		{req.Addresses.Proposer, &resp.Proposer},
	}

	var wg sync.WaitGroup
	for i := range entries {
		wg.Add(1)
		go func(addr string, result *dtos.AccountBalanceResult) {
			defer wg.Done()
			result.Address = addr
			callCtx, cancel := context.WithTimeout(ctx, rpcCallTimeout)
			defer cancel()
			bal, balErr := fetchBalance(callCtx, client, addr)
			if balErr != nil {
				logger.Warn("balance fetch failed", zap.String("address", addr), zap.Error(balErr))
				result.BalanceError = balErr.Error()
			} else {
				result.BalanceEth = bal
			}
		}(entries[i].addr, entries[i].result)
	}
	wg.Wait()

	return &entities.Response{
		Status:  http.StatusOK,
		Message: "Balances fetched successfully",
		Data:    resp,
	}, nil
}

// fetchBalancesParallel queries ETH balances for all OperatorKeyInfo entries concurrently.
func fetchBalancesParallel(ctx context.Context, client *ethclient.Client, infos []*dtos.OperatorKeyInfo) {
	var wg sync.WaitGroup
	for _, info := range infos {
		wg.Add(1)
		go func(entry *dtos.OperatorKeyInfo) {
			defer wg.Done()
			callCtx, cancel := context.WithTimeout(ctx, rpcCallTimeout)
			defer cancel()
			bal, balErr := fetchBalance(callCtx, client, entry.Address)
			if balErr != nil {
				logger.Warn("balance fetch failed", zap.String("address", entry.Address), zap.Error(balErr))
				entry.BalanceError = balErr.Error()
			} else {
				entry.BalanceEth = bal
			}
		}(info)
	}
	wg.Wait()
}

// fetchBalance returns the ETH balance for an address as a decimal ETH string.
func fetchBalance(ctx context.Context, client *ethclient.Client, address string) (string, error) {
	if !common.IsHexAddress(address) {
		return "", fmt.Errorf("invalid Ethereum address: %s", address)
	}

	wei, err := client.BalanceAt(ctx, common.HexToAddress(address), nil)
	if err != nil {
		return "", fmt.Errorf("BalanceAt %s: %w", address, err)
	}

	return weiToEth(wei), nil
}

// weiToEth converts a wei amount to a decimal ETH string with exactly weiDecimals (18)
// decimal places for consistent machine-parseable output (e.g. "1.000000000000000000").
// Callers that need human-readable display should trim trailing zeros.
func weiToEth(wei *big.Int) string {
	weiPerEth := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(weiDecimals), nil))
	eth := new(big.Float).Quo(new(big.Float).SetInt(wei), weiPerEth)
	return eth.Text('f', weiDecimals)
}
