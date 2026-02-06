package thanos

import (
	"context"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/tokamak-network/trh-backend/internal/consts"
	"github.com/tokamak-network/trh-backend/pkg/api/dtos"
	"github.com/tokamak-network/trh-backend/pkg/domain/entities"
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
			isMainnet := chainId.Uint64() == consts.EthereumMainnetChainID
			if req.Network == entities.DeploymentNetworkMainnet && !isMainnet {
				rpcCheck.Valid = false
				rpcCheck.Error = "Selected Mainnet but RPC is not Ethereum Mainnet (ChainID 1)"
			} else if req.Network != entities.DeploymentNetworkMainnet && isMainnet {
				rpcCheck.Valid = false
				rpcCheck.Error = "Selected Testnet but RPC is Ethereum Mainnet (ChainID 1)"
			}
			rpcCheck.Details = map[string]interface{}{"chainId": chainId.Uint64()}
		}
	}
	response.Checks["rpcConnectivity"] = rpcCheck
	if !rpcCheck.Valid {
		response.AllValid = false
	}

	// 2. AWS Credentials check
	awsCheck := dtos.ValidationCheckResult{Valid: true}
	if !trhSdkAws.IsAvailableRegion(req.AwsAccessKey, req.AwsSecretAccessKey, req.AwsRegion) {
		awsCheck.Valid = false
		awsCheck.Error = "Cannot access AWS region or invalid credentials"
	}
	response.Checks["awsRegionAccess"] = awsCheck
	if !awsCheck.Valid {
		response.AllValid = false
	}

	// 3. Mainnet Logic
	if req.Network == entities.DeploymentNetworkMainnet {
		// Parameter Sanity
		paramCheck := dtos.ValidationCheckResult{Valid: true}
		var paramErrors []string

		// Challenge Period must be at least 7 days (604800 seconds)
		if req.ChallengePeriod < consts.MainnetChallengePeriodSeconds {
			paramCheck.Valid = false
			paramErrors = append(paramErrors, "Challenge Period must be at least 7 days (604800s)")
		}
		// L2 Block Time must be at least 2 seconds (Optimism standard)
		if req.L2BlockTime < consts.MainnetMinL2BlockTimeSeconds {
			paramCheck.Valid = false
			paramErrors = append(paramErrors, "L2 Block Time must be at least 2s")
		}
		if len(paramErrors) > 0 {
			paramCheck.Error = strings.Join(paramErrors, "; ")
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

	// Calculate deployment gas cost
	var deploymentGasEth string
	if rpcCheck.Valid {
		gasPriceWei, err := client.SuggestGasPrice(ctx)
		if err == nil {
			// Estimate deployment cost: estimatedDeployContracts * gasPriceWei * 1.5 (margin)
			estimatedDeployContracts := new(big.Int).SetInt64(80_000_000)
			estimatedCost := new(big.Int).Mul(gasPriceWei, estimatedDeployContracts)
			// Apply 1.5x margin (multiply by 3, then divide by 2)
			estimatedCost.Mul(estimatedCost, big.NewInt(3))
			estimatedCost.Div(estimatedCost, big.NewInt(2))
			// Convert Wei to ETH (divide by 10^18)
			weiPerEth := new(big.Float).SetInt(big.NewInt(1e18))
			costEth := new(big.Float).Quo(new(big.Float).SetInt(estimatedCost), weiPerEth)
			deploymentGasEth = costEth.String()
		} else {
			deploymentGasEth = "error: failed to get gas price"
		}
	} else {
		deploymentGasEth = "error: invalid RPC connection"
	}
	response.EstimatedCost = &dtos.EstimatedCost{
		DeploymentGasEth: deploymentGasEth,
	}

	return response, nil
}
