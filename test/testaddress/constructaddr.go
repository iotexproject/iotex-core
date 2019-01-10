// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testaddress

import (
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// ConstructAddress constructs an iotex address
func ConstructAddress(chainID uint32, pubkey, prikey string) *iotxaddress.Address {
	pubk, err := keypair.DecodePublicKey(pubkey)
	if err != nil {
		log.L().Error("Failed to construct the address.", zap.Error(err))
	}
	prik, err := keypair.DecodePrivateKey(prikey)
	if err != nil {
		log.L().Error("Failed to construct the address.", zap.Error(err))
	}
	pkHash := keypair.HashPubKey(pubk)
	addr := address.New(chainID, pkHash[:])

	return &iotxaddress.Address{
		PublicKey:  pubk,
		PrivateKey: prik,
		RawAddress: addr.IotxAddress(),
	}
}
