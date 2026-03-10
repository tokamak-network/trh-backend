package thanos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
)

// RequiredFundingByNetwork defines the minimum required balances (in wei) per
// network and role. Values are placeholder estimates until product confirms
// exact thresholds.
var RequiredFundingByNetwork = map[entities.DeploymentNetwork]map[string]string{
	entities.DeploymentNetworkTestnet: {
		"admin":     "500000000000000000",  // 0.5 ETH
		"sequencer": "100000000000000000",  // 0.1 ETH
		"batcher":   "1000000000000000000", // 1 ETH
		"proposer":  "200000000000000000",  // 0.2 ETH
	},
	entities.DeploymentNetworkMainnet: {
		"admin":     "2000000000000000000", // 2 ETH
		"sequencer": "500000000000000000",  // 0.5 ETH
		"batcher":   "5000000000000000000", // 5 ETH
		"proposer":  "1000000000000000000", // 1 ETH
	},
}

// AccountFunding holds the funding state for a single role account.
type AccountFunding struct {
	Role      string `json:"role"`
	Address   string `json:"address"`
	Required  string `json:"required"`
	Current   string `json:"current"`
	Fulfilled bool   `json:"fulfilled"`
}

// FundingStatusResponse is the response shape for the funding status endpoint.
type FundingStatusResponse struct {
	StackID      string           `json:"stack_id"`
	Network      string           `json:"network"`
	Accounts     []AccountFunding `json:"accounts"`
	AllFulfilled bool             `json:"allFulfilled"`
	CheckedAt    string           `json:"checkedAt"`
}

// GetFundingStatus reads the stack config and queries L1 balances for each
// role account, comparing them against RequiredFundingByNetwork.
func (s *ThanosStackDeploymentService) GetFundingStatus(
	ctx context.Context,
	stackID string,
) (*FundingStatusResponse, error) {
	stack, err := s.stackRepo.GetStackByID(stackID)
	if err != nil {
		return nil, fmt.Errorf("stack not found: %w", err)
	}

	var cfg dtos.DeployThanosRequest
	if err := json.Unmarshal(stack.Config, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse stack config: %w", err)
	}

	required, ok := RequiredFundingByNetwork[stack.Network]
	if !ok {
		return nil, fmt.Errorf("no funding requirements defined for network %q", stack.Network)
	}

	roleAddresses := map[string]string{
		"admin":     cfg.AdminAccount,
		"sequencer": cfg.SequencerAccount,
		"batcher":   cfg.BatcherAccount,
		"proposer":  cfg.ProposerAccount,
	}

	for role, addr := range roleAddresses {
		if addr == "" {
			return nil, errors.New("missing address for role: " + role)
		}
	}

	client, err := ethclient.DialContext(ctx, cfg.L1RpcUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to L1 RPC: %w", err)
	}
	defer client.Close()

	roleOrder := []string{"admin", "sequencer", "batcher", "proposer"}
	accounts := make([]AccountFunding, 0, len(roleOrder))
	allFulfilled := true

	for _, role := range roleOrder {
		addr := roleAddresses[role]
		req := required[role]

		balance, err := client.BalanceAt(ctx, common.HexToAddress(addr), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to query balance for %s (%s): %w", role, addr, err)
		}

		reqInt := new(big.Int)
		reqInt.SetString(req, 10)

		fulfilled := balance.Cmp(reqInt) >= 0
		if !fulfilled {
			allFulfilled = false
		}

		accounts = append(accounts, AccountFunding{
			Role:      role,
			Address:   addr,
			Required:  req,
			Current:   balance.String(),
			Fulfilled: fulfilled,
		})
	}

	return &FundingStatusResponse{
		StackID:      stack.ID.String(),
		Network:      string(stack.Network),
		Accounts:     accounts,
		AllFulfilled: allFulfilled,
		CheckedAt:    time.Now().UTC().Format(time.RFC3339),
	}, nil
}
