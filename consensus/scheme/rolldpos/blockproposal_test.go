// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestNewBlockProposal(t *testing.T) {
	require := require.New(t)
	bp := newBlockProposal(nil, nil)
	require.NotNil(bp)
	require.Panics(func() { bp.Height() }, "block is nil")
	require.Panics(func() { bp.Proto() }, "block is nil")
	require.Panics(func() { bp.Hash() }, "block is nil")
	require.Panics(func() { bp.ProposerAddress() }, "block is nil")
	b := getBlock(t)
	bp2 := newBlockProposal(&b, nil)
	require.NotNil(bp2)
	require.Equal(uint64(123), bp2.Height())
	require.Equal(identityset.Address(0).String(), bp2.ProposerAddress())

	h, err := bp2.Hash()
	require.NoError(err)
	require.Equal(32, len(h))

	pro, err := bp2.Proto()
	require.NoError(err)

	bp3 := newBlockProposal(nil, nil)
	require.NoError(bp3.LoadProto(pro, block.NewDeserializer(0)))
	pro3, err := bp3.Proto()
	require.NoError(err)
	require.EqualValues(pro, pro3)
}
func getBlock(t *testing.T) block.Block {
	require := require.New(t)
	ts := &timestamppb.Timestamp{Seconds: 10, Nanos: 10}
	hcore := &iotextypes.BlockHeaderCore{
		Version:          1,
		Height:           123,
		Timestamp:        ts,
		PrevBlockHash:    []byte(""),
		TxRoot:           []byte(""),
		DeltaStateDigest: []byte(""),
		ReceiptRoot:      []byte(""),
	}
	header := block.Header{}
	require.NoError(header.LoadFromBlockHeaderProto(&iotextypes.BlockHeader{Core: hcore, ProducerPubkey: identityset.PrivateKey(0).PublicKey().Bytes()}))

	b := block.Block{Header: header}
	return b
}
