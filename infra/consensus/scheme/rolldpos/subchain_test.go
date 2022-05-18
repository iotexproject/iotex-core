// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestPutBlockToParentChain(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pubKey := identityset.PrivateKey(27).PublicKey()
	blk := block.Block{}
	blkpb := &iotextypes.Block{
		Header: &iotextypes.BlockHeader{
			Core: &iotextypes.BlockHeaderCore{
				Version:   version.ProtocolVersion,
				Height:    123456789,
				Timestamp: timestamppb.Now(),
			},
			ProducerPubkey: pubKey.Bytes(),
		},
		Body: &iotextypes.BlockBody{
			Actions: []*iotextypes.Action{
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_Transfer{
							Transfer: &iotextypes.Transfer{},
						},
						Version: version.ProtocolVersion,
						Nonce:   101,
					},
					SenderPubKey: pubKey.Bytes(),
					Signature:    action.ValidSig,
				},
				{
					Core: &iotextypes.ActionCore{
						Action: &iotextypes.ActionCore_Transfer{
							Transfer: &iotextypes.Transfer{},
						},
						Version: version.ProtocolVersion,
						Nonce:   102,
					},
					SenderPubKey: pubKey.Bytes(),
					Signature:    action.ValidSig,
				},
			},
		},
	}
	require.NoError(t, blk.ConvertFromBlockPb(blkpb))
	txRoot, err := blk.CalculateTxRoot()
	require.NoError(t, err)
	blkpb.Header.Core.TxRoot = txRoot[:]
	blk = block.Block{}
	require.NoError(t, blk.ConvertFromBlockPb(blkpb))
}
