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

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestPutBlock(t *testing.T) {
	assertPut := func(put *PutBlock) {
		assert.Equal(t, uint32(version.ProtocolVersion), put.version)
		assert.Equal(t, uint64(1), put.Nonce())
		assert.Equal(t, uint32(10001), put.ChainID())
		assert.Equal(t, uint64(10002), put.Height())
		assert.Equal(t, byteutil.BytesTo32B(hash.Hash256b([]byte{1})), put.hash)
		assert.Equal(t, byteutil.BytesTo32B(hash.Hash256b([]byte{2})), put.actionRoot)
		assert.Equal(t, byteutil.BytesTo32B(hash.Hash256b([]byte{3})), put.stateRoot)
		assert.Equal(t, testaddress.Addrinfo["producer"].RawAddress, put.ProducerAddress())
		assert.Equal(t, 2, len(put.endorsements))
		assert.Equal(t, testaddress.Addrinfo["alfa"].PublicKey, put.endorsements[0].endorser)
		assert.Equal(t, []byte{4}, put.endorsements[0].signature)
		assert.Equal(t, testaddress.Addrinfo["bravo"].PublicKey, put.endorsements[1].endorser)
		assert.Equal(t, []byte{5}, put.endorsements[1].signature)
		assert.Equal(t, uint64(10003), put.GasLimit())
		assert.Equal(t, big.NewInt(10004), put.GasPrice())
	}

	put1 := NewPutBlock(
		1,
		10001,
		10002,
		byteutil.BytesTo32B(hash.Hash256b([]byte{1})),
		byteutil.BytesTo32B(hash.Hash256b([]byte{2})),
		byteutil.BytesTo32B(hash.Hash256b([]byte{3})),
		testaddress.Addrinfo["producer"].RawAddress,
		[]*Endorsement{
			{
				endorser:  testaddress.Addrinfo["alfa"].PublicKey,
				signature: []byte{4},
			},
			{
				endorser:  testaddress.Addrinfo["bravo"].PublicKey,
				signature: []byte{5},
			},
		},
		10003,
		big.NewInt(10004),
	)
	assertPut(put1)

	putPb := put1.Proto()
	assert.NotNil(t, putPb)
	var put2 PutBlock
	put2.FromProto(putPb)
	assertPut(&put2)
}
