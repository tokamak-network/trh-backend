// Package keyderivation provides BIP44 HD wallet key derivation for Thanos Stack operator roles.
// Standard path: m/44'/60'/0'/0/{index}  (Ethereum BIP44)
//
//	index 0 → Admin
//	index 1 → Sequencer
//	index 2 → Batcher
//	index 3 → Proposer
package keyderivation

import (
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	bip32 "github.com/tyler-smith/go-bip32"
	bip39 "github.com/tyler-smith/go-bip39"
)

// RoleIndex defines BIP44 child indices for each operator role.
const (
	AdminIndex     uint32 = 0
	SequencerIndex uint32 = 1
	BatcherIndex   uint32 = 2
	ProposerIndex  uint32 = 3
)

// DerivedKey holds the private key (hex, no 0x prefix) and Ethereum address for one role.
type DerivedKey struct {
	PrivateKey string // 64-char hex string, no 0x prefix
	Address    string // checksummed 0x-prefixed Ethereum address
}

// DeriveOperatorKeys derives private keys and addresses for all four operator roles
// from a BIP39 mnemonic phrase using the standard Ethereum BIP44 path m/44'/60'/0'/0/{index}.
//
// passphrase is typically empty ("") for standard BIP39 derivation.
func DeriveOperatorKeys(mnemonic, passphrase string) (DerivedKey, DerivedKey, DerivedKey, DerivedKey, error) {
	zero := DerivedKey{}

	changePath, err := deriveChangePath(mnemonic, passphrase)
	if err != nil {
		return zero, zero, zero, zero, err
	}

	var keys [4]DerivedKey
	for i, idx := range []uint32{AdminIndex, SequencerIndex, BatcherIndex, ProposerIndex} {
		child, dErr := changePath.NewChildKey(idx)
		if dErr != nil {
			return zero, zero, zero, zero, fmt.Errorf("derive child key at index %d: %w", idx, dErr)
		}
		keys[i], dErr = bip32KeyToDerivedKey(child)
		if dErr != nil {
			return zero, zero, zero, zero, fmt.Errorf("encode key at index %d: %w", idx, dErr)
		}
	}
	return keys[0], keys[1], keys[2], keys[3], nil
}

// DeriveKeyAtIndex derives a single operator key at the given index.
func DeriveKeyAtIndex(mnemonic, passphrase string, index uint32) (DerivedKey, error) {
	changePath, err := deriveChangePath(mnemonic, passphrase)
	if err != nil {
		return DerivedKey{}, err
	}

	child, err := changePath.NewChildKey(index)
	if err != nil {
		return DerivedKey{}, fmt.Errorf("derive child at index %d: %w", index, err)
	}

	return bip32KeyToDerivedKey(child)
}

// deriveChangePath validates the mnemonic and returns the BIP44 change-level key m/44'/60'/0'/0.
func deriveChangePath(mnemonic, passphrase string) (*bip32.Key, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid BIP39 mnemonic")
	}
	seed := bip39.NewSeed(mnemonic, passphrase)
	defer func() {
		for i := range seed {
			seed[i] = 0
		}
	}()
	master, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, fmt.Errorf("derive master key: %w", err)
	}
	return derivePath(master)
}

// derivePath builds m/44'/60'/0'/0 from the master key.
func derivePath(master *bip32.Key) (*bip32.Key, error) {
	// m/44'
	purpose, err := master.NewChildKey(bip32.FirstHardenedChild + 44)
	if err != nil {
		return nil, fmt.Errorf("derive purpose (44'): %w", err)
	}
	// m/44'/60'
	coinType, err := purpose.NewChildKey(bip32.FirstHardenedChild + 60)
	if err != nil {
		return nil, fmt.Errorf("derive coin type (60'): %w", err)
	}
	// m/44'/60'/0'
	account, err := coinType.NewChildKey(bip32.FirstHardenedChild + 0)
	if err != nil {
		return nil, fmt.Errorf("derive account (0'): %w", err)
	}
	// m/44'/60'/0'/0
	change, err := account.NewChildKey(0)
	if err != nil {
		return nil, fmt.Errorf("derive change (0): %w", err)
	}
	return change, nil
}

// bip32KeyToDerivedKey converts a BIP32 leaf key to a DerivedKey with private key hex and address.
func bip32KeyToDerivedKey(k *bip32.Key) (DerivedKey, error) {
	ecKey, err := crypto.ToECDSA(k.Key)
	if err != nil {
		return DerivedKey{}, fmt.Errorf("parse ECDSA key: %w", err)
	}

	return DerivedKey{
		PrivateKey: hex.EncodeToString(crypto.FromECDSA(ecKey)),
		Address:    crypto.PubkeyToAddress(ecKey.PublicKey).Hex(),
	}, nil
}
