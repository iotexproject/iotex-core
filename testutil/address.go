// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"go.uber.org/zap"

	"github.com/iotexproject/go-ethereum/crypto"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
)

func NewTestAddress() address.Address {
	keyPair, err := crypto.GenerateKey()
	if err != nil {
		log.L().Panic("Error when generating key pair", zap.Error(err))
	}
	pkHash := keypair.HashPubKey(&keyPair.PublicKey)
	return address.New(pkHash[:])
}
