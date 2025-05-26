package utils

import (
	"context"

	"github.com/ethereum/go-ethereum/ethclient"
)

func GetChainIDFromRPC(l1RpcUrl string) (uint64, error) {
	client, err := ethclient.Dial(l1RpcUrl)
	if err != nil {
		return 0, err
	}

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return 0, err
	}

	return chainID.Uint64(), nil
}
