package model

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/tyler-smith/go-bip39"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
)

// Wallet stores the seed of wallet
type Wallet struct {
	MasterKey string 		`json:"masterKey"`
	Seed string 			`json:"seed"`
	NextDerivationIndex int	`json:"nextDerivationIndex"`
}

// NewWalletFromMnemonic Generate wallet from mnemonic
func NewWalletFromMnemonic(mnemonic string, passphrase string) (*Wallet, error) {

	if mnemonic == "" {
		return nil, errors.New("mnemonic is required")
	}

	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, errors.New("mnemonic is invalid")
	}

	seed, err := NewSeedFromMnemonic(mnemonic, passphrase)
	if err != nil {
		return nil, err
	}

	wallet, err := newWallet(seed)
	if err != nil {
		return nil, err
	}

	return wallet, nil
}

func newWallet(seed []byte) (*Wallet, error) {
	masterKey, err := hdkeychain.NewMaster(seed, &chaincfg.MainNetParams)
	if err != nil {
		return nil, err
	}
	masterKeyStr := masterKey.String()

	seedHexEncoded := hex.EncodeToString(seed)

	return &Wallet{
		MasterKey: 		 	 masterKeyStr,
		Seed:      		 	 seedHexEncoded,
		NextDerivationIndex: 0,
	}, nil
}

// NewSeedFromMnemonic returns a BIP-39 seed based on a BIP-39 mnemonic.
func NewSeedFromMnemonic(mnemonic string, passphrase string) ([]byte, error) {
	if mnemonic == "" {
		return nil, errors.New("mnemonic is required")
	}

	return bip39.NewSeedWithErrorChecking(mnemonic, passphrase)
}

// ReadWallet returns wallet JSON (for DEV only)
func ReadWallet(rootAddress string, ctx context.Context, req *logical.Request) (*Wallet, error) {
	walletPath := fmt.Sprintf("wallet/%s", rootAddress)
	entry, err := req.Storage.Get(ctx, walletPath)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, fmt.Errorf("wallet does not exist at %v", walletPath)
	}

	var wallet *Wallet
	err = entry.DecodeJSON(&wallet)
	if err != nil {
		return nil, errors.New("Fail to decode wallet to JSON format")
	}

	return wallet, nil
}

// Derive acctount from derivation path
func (w *Wallet) Derive(derivationPath string) (*Account, error) {
	path, err := hdwallet.ParseDerivationPath(derivationPath)
	if err != nil {
		return nil, err
	}

	address, err := w.deriveAddress(path)
	addressStr := address.String()

	// If an error occurred or no pinning was requested, return
	if err != nil {
		return &Account{}, err
	}

	URL := accounts.URL{
		Scheme: "",
		Path:   path.String(),
	}
	URLStr := URL.String()

	privateKey, err := w.derivePrivateKey(path)
	pritvateKeyStr := privateKeyHex(privateKey)

	publicKey, err := w.derivePublicKey(path)
	publicKeyStr := publicKeyHex(publicKey)

	account := &Account{
		Address:    addressStr,
		URL:        URLStr,
		PrivateKey: pritvateKeyStr,
		PublicKey:  publicKeyStr,
	}

	return account, nil
}

// Derive next account by incrementing derivation index
func (w *Wallet) DeriveNext(rootAddress string, ctx context.Context, req *logical.Request) (*Account, error) {
	account, err := w.Derive(fmt.Sprintf("m/44'/60'/%d'/0/0", w.NextDerivationIndex))
	if err != nil {
		return nil, err
	}

	w.NextDerivationIndex += 1
	entry, err := logical.StorageEntryJSON(fmt.Sprintf("wallet/%s", rootAddress), w)
	if err != nil {
		return nil, err
	}

	err = req.Storage.Put(ctx, entry)
	if err != nil {
		return nil, err
	}

	return account, nil
}

// PrivateKeyBytes returns the ECDSA private key in bytes format of the account.
func privateKeyBytes(privateKey *ecdsa.PrivateKey) []byte {
	return crypto.FromECDSA(privateKey)
}

// PrivateKeyHex return the ECDSA private key in hex string format of the account.
func privateKeyHex(privateKey *ecdsa.PrivateKey) string {
	privateKeyBytes := privateKeyBytes(privateKey)

	return hexutil.Encode(privateKeyBytes)[2:]
}

// PublicKeyBytes returns the ECDSA public key in bytes format of the account.
func publicKeyBytes(publicKey *ecdsa.PublicKey) []byte {
	return crypto.FromECDSAPub(publicKey)
}

// PublicKeyHex return the ECDSA public key in hex string format of the account.
func publicKeyHex(publicKey *ecdsa.PublicKey) string {
	publicKeyBytes := publicKeyBytes(publicKey)

	return hexutil.Encode(publicKeyBytes)[4:]
}

// ParseDerivationPath parses the derivation path in string format into []uint32
func ParseDerivationPath(path string) (accounts.DerivationPath, error) {
	return accounts.ParseDerivationPath(path)
}

// MustParseDerivationPath parses the derivation path in string format into
// []uint32 but will panic if it can't parse it.
func MustParseDerivationPath(path string) accounts.DerivationPath {
	parsed, err := accounts.ParseDerivationPath(path)
	if err != nil {
		panic(err)
	}

	return parsed
}

// DerivePrivateKey derives the private key of the derivation path.
func (w *Wallet) derivePrivateKey(path accounts.DerivationPath) (*ecdsa.PrivateKey, error) {
	var err error
	key, err := hdkeychain.NewKeyFromString(w.MasterKey)
	if err != nil {
		return nil, err
	}

	for _, n := range path {
		key, err = key.Derive(n)
		if err != nil {
			return nil, err
		}
	}

	privateKey, err := key.ECPrivKey()
	privateKeyECDSA := privateKey.ToECDSA()
	if err != nil {
		return nil, err
	}

	return privateKeyECDSA, nil
}

// DerivePublicKey derives the public key of the derivation path.
func (w *Wallet) derivePublicKey(path accounts.DerivationPath) (*ecdsa.PublicKey, error) {
	privateKeyECDSA, err := w.derivePrivateKey(path)
	if err != nil {
		return nil, err
	}

	publicKey := privateKeyECDSA.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("failed to get public key")
	}

	return publicKeyECDSA, nil
}

// DeriveAddress derives the account address of the derivation path.
func (w *Wallet) deriveAddress(path accounts.DerivationPath) (common.Address, error) {
	publicKeyECDSA, err := w.derivePublicKey(path)
	if err != nil {
		return common.Address{}, err
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	return address, nil
}
