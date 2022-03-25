// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_apicoreservice"
	"github.com/iotexproject/iotex-core/testutil"
)

var (
	testTransfer, _ = action.SignedTransfer(identityset.Address(28).String(),
		identityset.PrivateKey(28), 3, big.NewInt(10), []byte{}, testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64))

	testTransferHash, _ = testTransfer.Hash()
	testTransferPb      = testTransfer.Proto()

	testExecution, _ = action.SignedExecution(identityset.Address(29).String(),
		identityset.PrivateKey(29), 1, big.NewInt(0), testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64), []byte{})

	testExecutionHash, _ = testExecution.Hash()
	testExecutionPb      = testExecution.Proto()

	sendActionTests = []struct {
		// Arguments
		actionPb *iotextypes.Action
		// Expected Values
		actionHash string
	}{
		{
			testTransferPb,
			hex.EncodeToString(testTransferHash[:]),
		},
		{
			testExecutionPb,
			hex.EncodeToString(testExecutionHash[:]),
		},
	}
)

func TestGrpcServer_GetAccount(t *testing.T) {

}

func TestGrpcServer_GetActions(t *testing.T) {

}

func TestGrpcServer_GetAction(t *testing.T) {

}

func TestGrpcServer_GetActionsByAddress(t *testing.T) {

}

func TestGrpcServer_GetUnconfirmedActionsByAddress(t *testing.T) {

}

func TestGrpcServer_GetActionsByBlock(t *testing.T) {

}

func TestGrpcServer_GetBlockMetas(t *testing.T) {

}

func TestGrpcServer_GetBlockMeta(t *testing.T) {

}

func TestGrpcServer_GetChainMeta(t *testing.T) {

}

func TestGrpcServer_SendAction(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	grpcSvr := NewGRPCServer(core, testutil.RandomPort(), 10, 10)

	for _, test := range sendActionTests {
		core.EXPECT().SendAction(context.Background(), test.actionPb).Return(test.actionHash, nil)
		request := &iotexapi.SendActionRequest{Action: test.actionPb}
		res, err := grpcSvr.SendAction(context.Background(), request)
		require.NoError(err)
		require.Equal(test.actionHash, res.ActionHash)
	}
}

func TestGrpcServer_StreamLogs(t *testing.T) {

}

func TestGrpcServer_GetReceiptByAction(t *testing.T) {

}

func TestGrpcServer_GetServerMeta(t *testing.T) {

}

func TestGrpcServer_ReadContract(t *testing.T) {

}

func TestGrpcServer_SuggestGasPrice(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	grpcSvr := NewGRPCServer(core, testutil.RandomPort(), 10, 10)

	core.EXPECT().SuggestGasPrice().Return(uint64(1), nil)
	res, err := grpcSvr.SuggestGasPrice(context.Background(), &iotexapi.SuggestGasPriceRequest{})
	require.NoError(err)
	require.Equal(uint64(1), res.GasPrice)

	core.EXPECT().SuggestGasPrice().Return(uint64(0), errors.New("mock gas price error"))
	_, err = grpcSvr.SuggestGasPrice(context.Background(), &iotexapi.SuggestGasPriceRequest{})
	require.Contains(err.Error(), "mock gas price error")
}

func TestGrpcServer_EstimateGasForAction(t *testing.T) {

}

func TestGrpcServer_EstimateActionGasConsumption(t *testing.T) {

}

func TestGrpcServer_ReadUnclaimedBalance(t *testing.T) {

}

func TestGrpcServer_TotalBalance(t *testing.T) {

}

func TestGrpcServer_AvailableBalance(t *testing.T) {

}

func TestGrpcServer_ReadCandidatesByEpoch(t *testing.T) {

}

func TestGrpcServer_ReadBlockProducersByEpoch(t *testing.T) {

}

func TestGrpcServer_ReadActiveBlockProducersByEpoch(t *testing.T) {

}

func TestGrpcServer_ReadRollDPoSMeta(t *testing.T) {

}

func TestGrpcServer_ReadEpochCtx(t *testing.T) {

}

func TestGrpcServer_GetEpochMeta(t *testing.T) {

}

func TestGrpcServer_GetRawBlocks(t *testing.T) {

}

func TestGrpcServer_GetLogs(t *testing.T) {

}

func TestGrpcServer_GetElectionBuckets(t *testing.T) {

}

func TestGrpcServer_GetActionByActionHash(t *testing.T) {

}

func TestGrpcServer_GetTransactionLogByActionHash(t *testing.T) {

}

func TestGrpcServer_GetEvmTransfersByBlockHeight(t *testing.T) {

}

func TestGrpcServer_GetActPoolActions(t *testing.T) {

}

func TestGrpcServer_GetEstimateGasSpecial(t *testing.T) {

}

func TestChainlinkErrTest(t *testing.T) {

}

func TestGrpcServer_TraceTransactionStructLogs(t *testing.T) {

}
