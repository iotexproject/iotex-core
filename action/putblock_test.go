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
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestPutBlock(t *testing.T) {
	addr := testaddress.Addrinfo["producer"]
	addr2 := testaddress.Addrinfo["echo"]
	assertPB := func(pb *PutBlock) {
		assert.Equal(t, uint32(version.ProtocolVersion), pb.version)
		assert.Equal(t, uint64(1), pb.Nonce())
		assert.Equal(t, addr.RawAddress, pb.ProducerAddress())
		assert.Equal(t, addr2.RawAddress, pb.SubChainAddress())
		assert.Equal(t, uint64(10001), pb.Height())
		assert.Equal(t, byteutil.BytesTo32B([]byte("10002")), pb.Roots()["10002"])
		assert.Equal(t, uint64(10003), pb.GasLimit())
		assert.Equal(t, big.NewInt(10004), pb.GasPrice())
	}
	roots := make(map[string]hash.Hash32B)
	roots["10002"] = byteutil.BytesTo32B([]byte("10002"))
	pb := NewPutBlock(
		1,
		addr2.RawAddress,
		addr.RawAddress,
		10001,
		roots,
		10003,
		big.NewInt(10004),
	)
	require.NotNil(t, pb)
	assertPB(pb)

	putBlockPb := pb.Proto()
	require.NotNil(t, putBlockPb)
	npb := &PutBlock{}
	assert.NoError(t, npb.LoadProto(putBlockPb))
	require.NotNil(t, npb)
	assertPB(npb)
}

func TestPutBlockByteStream(t *testing.T) {
	addr := testaddress.Addrinfo["producer"]
	addr2 := testaddress.Addrinfo["echo"]
	roots := make(map[string]hash.Hash32B)
	roots["10002"] = byteutil.BytesTo32B([]byte("10002"))
	roots["10003"] = byteutil.BytesTo32B([]byte("10003"))
	roots["10004"] = byteutil.BytesTo32B([]byte("10004"))
	roots["10005"] = byteutil.BytesTo32B([]byte("10005"))
	pb := NewPutBlock(
		1,
		addr2.RawAddress,
		addr.RawAddress,
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
