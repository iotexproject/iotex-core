// Copyright (c) 2019 IoTeX Foundation
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
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/testutil"
)

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
		Caller:   identityset.Address(27),
		Producer: identityset.Address(27),
		GasLimit: testutil.TestGasLimit,
	})

	retval, receipt, err := ExecuteContract(ctx, sm, e, cm, NewHeightChange(0, 0))
	require.Nil(t, retval)
	require.Nil(t, receipt)
	require.Error(t, err)
}
