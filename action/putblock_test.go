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
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestPutBlock(t *testing.T) {
	addr2 := testaddress.Addrinfo["echo"]
	assertPB := func(pb *PutBlock) {
		assert.Equal(t, uint32(version.ProtocolVersion), pb.version)
		assert.Equal(t, uint64(1), pb.Nonce())
		assert.Equal(t, addr2.String(), pb.SubChainAddress())
		assert.Equal(t, uint64(10001), pb.Height())
		assert.Equal(t, hash.BytesToHash256([]byte("10002")), pb.Roots()["10002"])
		assert.Equal(t, uint64(10003), pb.GasLimit())
		assert.Equal(t, big.NewInt(10004), pb.GasPrice())
	}
	roots := make(map[string]hash.Hash256)
	roots["10002"] = hash.BytesToHash256([]byte("10002"))
	pb := NewPutBlock(
		1,
		addr2.String(),
		10001,
		roots,
		10003,
		big.NewInt(10004),
	)
	require.NotNil(t, pb)
	assertPB(pb)
}

func TestPutBlockProto(t *testing.T) {
	addr2 := testaddress.Addrinfo["echo"]
	roots := make(map[string]hash.Hash256)
	roots["10002"] = hash.BytesToHash256([]byte("10002"))
	pb := &PutBlock{
		subChainAddress: addr2.String(),
		height:          10001,
		roots:           roots,
	}
	assertPB := func(pb *PutBlock) {
		assert.Equal(t, addr2.String(), pb.SubChainAddress())
		assert.Equal(t, uint64(10001), pb.Height())
		assert.Equal(t, hash.BytesToHash256([]byte("10002")), pb.Roots()["10002"])
	}
	putBlockPb := pb.Proto()
	require.NotNil(t, putBlockPb)
	npb := &PutBlock{}
	assert.NoError(t, npb.LoadProto(putBlockPb))
	require.NotNil(t, npb)
	assertPB(npb)
}

func TestPutBlockByteStream(t *testing.T) {
	addr := testaddress.Addrinfo["producer"]
	roots := make(map[string]hash.Hash256)
	roots["10002"] = hash.BytesToHash256([]byte("10002"))
	roots["10003"] = hash.BytesToHash256([]byte("10003"))
	roots["10004"] = hash.BytesToHash256([]byte("10004"))
	roots["10005"] = hash.BytesToHash256([]byte("10005"))
	pb := NewPutBlock(
		1,
		addr.String(),
		10001,
		roots,
		10003,
		big.NewInt(10004),
	)
	b := pb.ByteStream()

	for i := 0; i < 10; i++ {
		assert.Equal(t, b, pb.ByteStream())
	}
}
