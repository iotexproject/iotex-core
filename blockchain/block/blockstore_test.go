// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestStoreProto(t *testing.T) {
	require := require.New(t)
	store, err := makeStore()
	require.NoError(err)

	storeProto := store.ToProto()

	require.NotNil(storeProto)
	require.Equal(0, len(storeProto.Receipts))

	require.NoError(store.FromProto(storeProto))
}

func TestStoreSerDer(t *testing.T) {
	require := require.New(t)
	store, err := makeStore()
	require.NoError(err)

	ser, err := store.Serialize()
	require.NoError(err)

	store1 := &Store{}
	require.NoError(store1.Deserialize(ser))
	require.Equal(store1.Block.height, store.Block.height)
	require.Equal(store1.Block.Header.prevBlockHash, store.Block.Header.prevBlockHash)
	require.Equal(store1.Block.Header.blockSig, store.Block.Header.blockSig)
}

func makeStore() (Store, error) {
	receipts := []*action.Receipt{
		{
			BlockHeight: 1,
		},
		{
			BlockHeight: 2,
		},
	}
	nblk, err := NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(testutil.TimestampNow()).
		SetReceipts(receipts).
		SignAndBuild(identityset.PrivateKey(29))

	if err != nil {
		return Store{}, err
	}

	return Store{
		Block: &nblk,
	}, nil
}
