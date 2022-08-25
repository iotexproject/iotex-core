// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
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
	store1, err := bd.FromBlockStoreProto(storeProto)
	require.NoError(err)

	require.Equal(store1.Block.height, store.Block.height)
	require.Equal(store1.Block.Header.prevBlockHash, store.Block.Header.prevBlockHash)
	require.Equal(store1.Block.Header.blockSig, store.Block.Header.blockSig)
}
