// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestActionBuilder(t *testing.T) {
	srcAddr := testaddress.Addrinfo["producer"]
	dstAddr := testaddress.Addrinfo["echo"]
	srcPubKey := &testaddress.Keyinfo["producer"].PublicKey
	bd := &Builder{}
	act := bd.SetVersion(version.ProtocolVersion).
		SetNonce(2).
		SetSourceAddress(srcAddr.Bech32()).
		SetSourcePublicKey(srcPubKey).
		SetDestinationAddress(dstAddr.Bech32()).
		SetGasLimit(10003).
		SetGasPrice(big.NewInt(10004)).
		Build()

	assert.Equal(t, uint32(version.ProtocolVersion), act.Version())
	assert.Equal(t, uint64(2), act.Nonce())
	assert.Equal(t, srcAddr.Bech32(), act.SrcAddr())
	assert.Equal(t, srcPubKey, act.SrcPubkey())
	assert.Equal(t, dstAddr.Bech32(), act.DstAddr())
	assert.Equal(t, uint64(10003), act.GasLimit())
	assert.Equal(t, big.NewInt(10004), act.GasPrice())
}
