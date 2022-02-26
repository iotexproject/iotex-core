// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
)

type (
	// Erc721Token erc721 token interface
	Erc721Token interface {
		Contract
		CreateToken(string, string) (string, error)
		Transfer(string, string, string, string, string) (string, error)
	}

	erc721Token struct {
		Contract
	}
)

// NewErc721Token creates a new Erc721Token
func NewErc721Token(exp string) Erc721Token {
	return &erc721Token{Contract: NewContract(exp)}
}

func (f *erc721Token) CreateToken(tokenid, creditor string) (string, error) {
	TokenID, ok := new(big.Int).SetString(tokenid, 10)
	if !ok {
		return "", errors.Errorf("invalid tokenid = %s", tokenid)
	}
	addrCreditor, err := address.FromString(creditor)
	if err != nil {
		return "", errors.Errorf("invalid creditor address = %s", creditor)
	}
	owner := f.RunAsOwner()
	h, err := owner.Call(CreateTo, addrCreditor.Bytes(), TokenID.Bytes())
	if err != nil {
		return h, errors.Wrapf(err, "call failed to create")
	}

	if _, err := f.CheckCallResult(h); err != nil {
		return h, errors.Wrapf(err, "check failed to create")
	}
	return h, nil
}

//
func (f *erc721Token) Transfer(token, sender, prvkey, receiver string, tokenid string) (string, error) {
	TokenID, ok := new(big.Int).SetString(tokenid, 10)
	if !ok {
		return "", errors.Errorf("invalid tokenid = %s", tokenid)
	}
	from, err := address.FromString(sender)
	if err != nil {
		return "", errors.Errorf("invalid account address = %s", sender)
	}
	addrReceiver, err := address.FromString(receiver)
	if err != nil {
		return "", errors.Errorf("invalid account address = %s", receiver)
	}
	// transfer to receiver
	h, err := f.SetAddress(token).
		SetExecutor(sender).
		SetPrvKey(prvkey).
		Call(TransferFrom, from.Bytes(), addrReceiver.Bytes(), TokenID.Bytes())
	if err != nil {
		return h, errors.Wrap(err, "call transfer failed")
	}

	if _, err := f.CheckCallResult(h); err != nil {
		return h, errors.Wrap(err, "check transfer failed")
	}
	return h, nil
}
