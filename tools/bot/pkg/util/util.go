// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"bytes"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
)

func GetPrivateKey(walletPath, addr, password string) (crypto.PrivateKey, error) {
	ks := keystore.NewKeyStore(walletPath,
		keystore.StandardScryptN, keystore.StandardScryptP)

	from, err := address.FromString(addr)
	if err != nil {
		return nil, err
	}
	for _, account := range ks.Accounts() {
		if bytes.Equal(from.Bytes(), account.Address.Bytes()) {
			return crypto.KeystoreToPrivateKey(account, password)
		}
	}
	return nil, errors.New("src address not found")
}
