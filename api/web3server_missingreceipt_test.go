// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// getBlockWithTransactions must not panic when a block carries more actions
// than receipts; detailed transactions without a corresponding receipt are
// skipped rather than indexing receipts[i] out of bounds.
func TestGetBlockWithTransactionsMissingReceipts(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	core.EXPECT().EVMNetworkID().Return(uint32(0)).AnyTimes()
	web3svr := &web3Handler{core, nil, _defaultBatchRequestLimit}

	// three distinct signed transfers (different nonces => different hashes)
	tsf1, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), 1, big.NewInt(10), []byte{}, 100000, big.NewInt(0))
	r.NoError(err)
	tsf2, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), 2, big.NewInt(10), []byte{}, 100000, big.NewInt(0))
	r.NoError(err)
	tsf3, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), 3, big.NewInt(10), []byte{}, 100000, big.NewInt(0))
	r.NoError(err)

	h1, err := tsf1.Hash()
	r.NoError(err)
	h2, err := tsf2.Hash()
	r.NoError(err)

	// only two receipts for three actions; receipts align with tsf1/tsf2
	receipts := []*action.Receipt{
		{BlockHeight: 1, ActionHash: h1},
		{BlockHeight: 1, ActionHash: h2},
	}
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SetVersion(111).
		SetPrevBlockHash(hash.ZeroHash256).
		SetTimeStamp(time.Now()).
		SetReceipts(receipts).
		AddActions(tsf1, tsf2, tsf3).
		SignAndBuild(identityset.PrivateKey(0))
	r.NoError(err)

	t.Run("detailed skips receipt-less action without panic", func(t *testing.T) {
		r.NotPanics(func() {
			res, err := web3svr.getBlockWithTransactions(&blk, receipts, true)
			r.NoError(err)
			r.NotNil(res)
			// tsf3 has no receipt and is omitted; only two detailed txs returned
			r.Len(res.transactions, 2)
		})
	})

	t.Run("non-detailed lists all action hashes", func(t *testing.T) {
		// the hash-only branch does not touch receipts and returns every action
		res, err := web3svr.getBlockWithTransactions(&blk, receipts, false)
		r.NoError(err)
		r.Len(res.transactions, 3)
	})
}
