// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestLogReceipt(t *testing.T) {
	require := require.New(t)
	log := action.Log{Address: "abcde", Data: []byte("12345"), BlockNumber: 5, Index: 6}
	topic := hash.Hash256b([]byte("12345"))
	log.Topics = []hash.Hash256{topic}
	log.TxnHash = hash.Hash256b([]byte("11111"))
	s, err := log.Serialize()
	require.NoError(err)
	actuallog := action.Log{}
	require.NoError(actuallog.Deserialize(s))
	require.Equal(log.Address, actuallog.Address)
	require.Equal(log.Topics[0], actuallog.Topics[0])
	require.Equal(len(log.Topics), len(actuallog.Topics))
	require.Equal(log.Data, actuallog.Data)
	require.Equal(log.BlockNumber, actuallog.BlockNumber)
	require.Equal(log.TxnHash, actuallog.TxnHash)
	require.Equal(log.Index, actuallog.Index)

	receipt := action.Receipt{ReturnValue: []byte("12345"), Status: 5, GasConsumed: 6, ContractAddress: "aaaaa", Logs: []*action.Log{&log}}
	receipt.ActHash = hash.Hash256b([]byte("33333"))
	s, err = receipt.Serialize()
	require.NoError(err)
	actualReceipt := action.Receipt{}
	require.NoError(actualReceipt.Deserialize(s))
	require.Equal(receipt.ReturnValue, actualReceipt.ReturnValue)
	require.Equal(receipt.Status, actualReceipt.Status)
	require.Equal(receipt.GasConsumed, actualReceipt.GasConsumed)
	require.Equal(receipt.ContractAddress, actualReceipt.ContractAddress)
	require.Equal(receipt.Logs[0], actualReceipt.Logs[0])
	require.Equal(len(receipt.Logs), len(actualReceipt.Logs))
	require.Equal(receipt.ActHash, actualReceipt.ActHash)
}

func TestExecuteContractFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cm := mock_chainmanager.NewMockChainManager(ctrl)
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	store := db.NewMemKVStore()
	sm.EXPECT().GetDB().Return(store).AnyTimes()
	cb := db.NewCachedBatch()
	sm.EXPECT().GetCachedBatch().Return(cb).AnyTimes()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).Return(state.ErrStateNotExist).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	sm.EXPECT().Snapshot().Return(1).AnyTimes()

	e, err := action.NewExecution(
		"",
		1,
		big.NewInt(0),
		testutil.TestGasLimit,
		big.NewInt(10),
		nil,
	)
	require.NoError(t, err)

	ctx := protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
		Caller:   testaddress.Addrinfo["producer"],
		Producer: testaddress.Addrinfo["producer"],
		GasLimit: testutil.TestGasLimit,
	})

	receipt, err := ExecuteContract(ctx, sm, e, cm)
	require.NotNil(t, receipt)
	assert.Equal(t, action.FailureReceiptStatus, receipt.Status)
	require.NoError(t, err)
}
