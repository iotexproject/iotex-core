// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestStoreProto(t *testing.T) {
	require := require.New(t)
	store, err := makeStore()
	require.NoError(err)

	storeProto := store.ToProto()

	require.NotNil(storeProto)
	require.Equal(0, len(storeProto.Receipts))

}

func makeStore() (*Store, error) {
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
		return nil, err
	}

	return &Store{
		Block: &nblk,
	}, nil
}
