// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

import (
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestBlockDeserializer(t *testing.T) {
	r := require.New(t)
	bd := Deserializer{}
	blk, err := bd.FromBlockProto(&_pbBlock)
	r.NoError(err)
	body, err := bd.fromBodyProto(_pbBlock.Body)
	r.NoError(err)
	r.Equal(body, blk.Body)

	txHash, err := blk.CalculateTxRoot()
	r.NoError(err)
	blk.Header.txRoot = txHash
	blk.Header.receiptRoot = hash.Hash256b(([]byte)("test"))
	raw, err := blk.Serialize()
	r.NoError(err)

	newblk, err := (&Deserializer{}).DeserializeBlock(raw)
	r.NoError(err)
	r.Equal(blk, newblk)
	r.Equal(_pbBlock.Body.Actions[0].Core.ChainID, blk.Actions[0].ChainID())
	r.Equal(_pbBlock.Body.Actions[1].Core.ChainID, blk.Actions[1].ChainID())
}

func TestBlockStoreDeserializer(t *testing.T) {
	require := require.New(t)
	store, err := makeStore()
	require.NoError(err)

	storeProto := store.ToProto()

	require.NotNil(storeProto)

	bd := Deserializer{}
	store1, err := bd.BlockFromBlockStoreProto(storeProto)
	require.NoError(err)

	require.Equal(store1.height, store.Block.height)
	require.Equal(store1.Header.prevBlockHash, store.Block.Header.prevBlockHash)
	require.Equal(store1.Header.blockSig, store.Block.Header.blockSig)
}

func TestBlockDeserializerNilBody(t *testing.T) {
	r := require.New(t)
	bd := Deserializer{}

	pbBlock := proto.Clone(&_pbBlock).(*iotextypes.Block)
	pbBlock.Body = nil
	_, err := bd.FromBlockProto(pbBlock)
	r.ErrorContains(err, "block body is nil")

	raw, err := proto.Marshal(pbBlock)
	r.NoError(err)
	_, err = bd.DeserializeBlock(raw)
	r.ErrorContains(err, "block body is nil")

	_, err = bd.BlockFromBlockStoreProto(&iotextypes.BlockStore{})
	r.Error(err)
	_, err = bd.ReceiptsFromBlockStoreProto(&iotextypes.BlockStore{})
	r.NoError(err)

	// an empty body must still round-trip as a non-nil body
	pbBlock.Body = &iotextypes.BlockBody{}
	raw, err = proto.Marshal(pbBlock)
	r.NoError(err)
	decoded := &iotextypes.Block{}
	r.NoError(proto.Unmarshal(raw, decoded))
	r.NotNil(decoded.GetBody())
	blk, err := bd.FromBlockProto(decoded)
	r.NoError(err)
	r.Empty(blk.Actions)
}
