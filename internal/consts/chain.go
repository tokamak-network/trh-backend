package consts

import "math/big"

const L1_BLOCK_TIME = 12 // Ethereum L1 block time in seconds

// Mainnet validation constants
const (
	// MainnetChallengePeriodSeconds is the required challenge period for mainnet (7 days)
	MainnetChallengePeriodSeconds = 604800

	// MainnetMinL2BlockTimeSeconds is the minimum L2 block time for mainnet
	MainnetMinL2BlockTimeSeconds = 2

	// EthereumMainnetChainID is the chain ID for Ethereum mainnet
	EthereumMainnetChainID = 1
)

// Minimum balance requirements (Wei)
var (
	// Testnet/Devnet minimum balances
	MinBalanceAdminTestnet     = big.NewInt(500000000000000000) // 0.5 ETH
	MinBalanceSequencerTestnet = big.NewInt(10000000000000000)  // 0.01 ETH
	MinBalanceBatcherTestnet   = big.NewInt(10000000000000000)  // 0.01 ETH
	MinBalanceProposerTestnet  = big.NewInt(10000000000000000)  // 0.01 ETH

	// Mainnet minimum balances
	MinBalanceAdminMainnet     = big.NewInt(1000000000000000000) // 1 ETH
	MinBalanceSequencerMainnet = big.NewInt(0)                   // 0 ETH
	MinBalanceBatcherMainnet   = big.NewInt(1000000000000000000) // 1 ETH
	MinBalanceProposerMainnet  = big.NewInt(1000000000000000000) // 1 ETH
)
