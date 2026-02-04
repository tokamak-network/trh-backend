package thanos

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	trhSdkAws "github.com/tokamak-network/trh-sdk/pkg/cloud-provider/aws"
)

func (s *ThanosStackDeploymentService) ValidateDeployment(ctx context.Context, req *dtos.ValidateDeploymentRequest) (*dtos.ValidateDeploymentResponse, error) {
	response := &dtos.ValidateDeploymentResponse{
		AllValid: true,
		Checks:   make(map[string]dtos.ValidationCheckResult),
	}

	// 1. Check L1 RPC Connectivity & ChainID
	rpcCheck := dtos.ValidationCheckResult{Valid: true}
	client, err := ethclient.DialContext(ctx, req.L1RpcUrl)
	if err != nil {
		rpcCheck.Valid = false
		rpcCheck.Error = "Failed to connect to RPC: " + err.Error()
	} else {
		chainId, err := client.ChainID(ctx)
		if err != nil {
			rpcCheck.Valid = false
			rpcCheck.Error = "Failed to get ChainID: " + err.Error()
		} else {
			// Validate Network Match
			isMainnet := chainId.Uint64() == 1
			if string(req.Network) == "Mainnet" && !isMainnet {
				rpcCheck.Valid = false
				rpcCheck.Error = "Selected Mainnet but RPC is not Ethereum Mainnet (ChainID 1)"
			}
			rpcCheck.Details = map[string]interface{}{"chainId": chainId.Uint64()}
		}
	}
	response.Checks["rpcConnectivity"] = rpcCheck
	if !rpcCheck.Valid {
		response.AllValid = false
	}

	// 2. Check Account Balances (if RPC is valid)
	if rpcCheck.Valid {
		accounts := map[string]string{
			"admin":     req.AdminAddress,
			"sequencer": req.SequencerAddress,
			"batcher":   req.BatcherAddress,
			"proposer":  req.ProposerAddress,
		}

		balanceCheck := dtos.ValidationCheckResult{Valid: true, Details: make(map[string]interface{})}
		details := make(map[string]interface{})

		// Minimum requirements (Wei)
		// Default (Testnet/Devnet): Admin 0.5 ETH, others 0.01 ETH
		minBalances := map[string]*big.Int{
			"admin":     big.NewInt(500000000000000000), // 0.5 ETH
			"sequencer": big.NewInt(10000000000000000),  // 0.01 ETH
			"batcher":   big.NewInt(10000000000000000),  // 0.01 ETH
			"proposer":  big.NewInt(10000000000000000),  // 0.01 ETH
		}

		// Mainnet requirements: Admin 1 ETH, Proposer 1 ETH, Batcher 1 ETH, Sequencer 0 ETH
		if string(req.Network) == "Mainnet" {
			minBalances["admin"] = big.NewInt(1000000000000000000)    // 1 ETH
			minBalances["sequencer"] = big.NewInt(0)                  // 0 ETH
			minBalances["batcher"] = big.NewInt(1000000000000000000)  // 1 ETH
			minBalances["proposer"] = big.NewInt(1000000000000000000) // 1 ETH
		}

		for role, addrStr := range accounts {
			if !common.IsHexAddress(addrStr) {
				balanceCheck.Valid = false
				balanceCheck.Error = "Invalid address format"
				details[role] = map[string]interface{}{"error": "invalid address"}
				continue
			}
			addr := common.HexToAddress(addrStr)
			bal, err := client.BalanceAt(ctx, addr, nil)
			if err != nil {
				balanceCheck.Valid = false
				balanceCheck.Error = "Failed to fetch balance"
				details[role] = map[string]interface{}{"error": err.Error()}
			} else {
				isSufficient := bal.Cmp(minBalances[role]) >= 0
				details[role] = map[string]interface{}{
					"balance":    bal.String(),
					"required":   minBalances[role].String(),
					"sufficient": isSufficient,
				}
				if !isSufficient {
					balanceCheck.Valid = false
					// Accumulate errors?
					if balanceCheck.Error == "" {
						balanceCheck.Error = "Insufficient balance for " + role
					} else {
						balanceCheck.Error += ", " + role
					}
				}
			}
		}
		balanceCheck.Details = details
		response.Checks["accountBalances"] = balanceCheck
		if !balanceCheck.Valid {
			response.AllValid = false
		}
	}

	// 3. AWS Credentials check
	awsCheck := dtos.ValidationCheckResult{Valid: true}
	if !trhSdkAws.IsAvailableRegion(req.AwsAccessKey, req.AwsSecretAccessKey, req.AwsRegion) {
		awsCheck.Valid = false
		awsCheck.Error = "Cannot access AWS region or invalid credentials"
	}
	response.Checks["awsRegionAccess"] = awsCheck
	if !awsCheck.Valid {
		response.AllValid = false
	}

	// 4. Mainnet Logic
	if string(req.Network) == "Mainnet" {
		// Parameter Sanity
		paramCheck := dtos.ValidationCheckResult{Valid: true}
		// Challenge Period must be exactly 7 days (604800 seconds)
		if req.ChallengePeriod != 604800 {
			paramCheck.Valid = false
			paramCheck.Error = "Challenge Period must be exactly 7 days (604800s)"
		}
		// L2 Block Time must be at least 2 seconds (Optimism standard)
		if req.L2BlockTime < 2 {
			paramCheck.Valid = false
			paramCheck.Error = "L2 Block Time < 2s"
		}
		response.Checks["parameterSanity"] = paramCheck
		if !paramCheck.Valid {
			response.AllValid = false
		}

		// Confirmation
		confCheck := dtos.ValidationCheckResult{Valid: true}
		if req.MainnetConfirmation == nil ||
			!req.MainnetConfirmation.AcknowledgedIrreversibility ||
			!req.MainnetConfirmation.AcknowledgedCosts ||
			!req.MainnetConfirmation.AcknowledgedRisks {
			confCheck.Valid = false
			confCheck.Error = "Mainnet confirmation incomplete"
		}
		response.Checks["mainnetConfirmation"] = confCheck
		if !confCheck.Valid {
			response.AllValid = false
		}
	}

	// Dummy Cost
	response.EstimatedCost = &dtos.EstimatedCost{
		DeploymentGasEth:   "0.05",
		MonthlyAwsEth:      "0.10",
		TotalFirstMonthEth: "0.15",
	}

	return response, nil
}
