package dtos

// DeriveKeysRequest is the payload for POST /stacks/thanos/derive-keys.
// The seed phrase is used in-memory only and is never persisted.
type DeriveKeysRequest struct {
	// SeedPhrase is a 12- or 24-word BIP39 mnemonic.
	SeedPhrase string `json:"seedPhrase" binding:"required"`
	// BIP39Passphrase is an optional extra passphrase (usually empty).
	BIP39Passphrase string `json:"bip39Passphrase,omitempty"`
	// L1RpcUrl, when provided, triggers an ETH balance check for the derived addresses.
	L1RpcUrl string `json:"l1RpcUrl,omitempty"`
}

// OperatorKeyInfo holds the derived key material for one operator role.
type OperatorKeyInfo struct {
	// PrivateKey is the raw hex private key (no 0x prefix).
	PrivateKey string `json:"privateKey"`
	// Address is the checksummed Ethereum address (0x-prefixed).
	Address string `json:"address"`
	// BalanceEth is the on-chain ETH balance as a decimal string (e.g. "0.5").
	// Only populated when L1RpcUrl is provided and the query succeeds.
	BalanceEth string `json:"balanceEth,omitempty"`
	// BalanceError describes why the balance query failed, if it did.
	BalanceError string `json:"balanceError,omitempty"`
}

// DeriveKeysResponse holds the derived operator key material.
type DeriveKeysResponse struct {
	Admin     OperatorKeyInfo `json:"admin"`
	Sequencer OperatorKeyInfo `json:"sequencer"`
	Batcher   OperatorKeyInfo `json:"batcher"`
	Proposer  OperatorKeyInfo `json:"proposer"`
}

// CheckAccountBalancesRequest is the payload for POST /stacks/thanos/check-account-balances.
type CheckAccountBalancesRequest struct {
	L1RpcUrl  string              `json:"l1RpcUrl"  binding:"required"`
	Addresses OperatorAddresses   `json:"addresses" binding:"required"`
}

// OperatorAddresses contains the Ethereum address for each operator role.
type OperatorAddresses struct {
	Admin     string `json:"admin"     binding:"required"`
	Sequencer string `json:"sequencer" binding:"required"`
	Batcher   string `json:"batcher"   binding:"required"`
	Proposer  string `json:"proposer"  binding:"required"`
}

// AccountBalanceResult holds the ETH balance for a single account.
type AccountBalanceResult struct {
	Address      string `json:"address"`
	BalanceEth   string `json:"balanceEth,omitempty"`   // decimal string, e.g. "1.234567890123456789"
	BalanceError string `json:"balanceError,omitempty"` // non-empty when the balance query failed
}

// CheckAccountBalancesResponse holds balances for all operator roles.
type CheckAccountBalancesResponse struct {
	Admin     AccountBalanceResult `json:"admin"`
	Sequencer AccountBalanceResult `json:"sequencer"`
	Batcher   AccountBalanceResult `json:"batcher"`
	Proposer  AccountBalanceResult `json:"proposer"`
}
