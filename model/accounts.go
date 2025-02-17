package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/vault/sdk/logical"
)

// Account is derived from seed
type Account struct {
	Address    string `json:"address"`
	URL        string `json:"url"`
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
}

// ReadAccount returns the account JSON
func ReadAccount(accountAddress string, ctx context.Context, req *logical.Request) (*Account, error) {
	accountPath := fmt.Sprintf("account/%s", accountAddress)
	entry, err := req.Storage.Get(ctx, accountPath)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, fmt.Errorf("account does not exist at %v", accountPath)
	}

	var account *Account
	err = entry.DecodeJSON(&account)
	if err != nil {
		return nil, errors.New("Fail to decode account to JSON format")
	}

	return account, nil
}
