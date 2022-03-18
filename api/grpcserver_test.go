// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/mock/mock_apicoreservice"
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
	// require := require.New(t)
	// ctrl := gomock.NewController(t)

	// chain := mock_blockchain.NewMockBlockchain(ctrl)
	// ap := mock_actpool.NewMockActPool(ctrl)

	// broadcastHandlerCount := 0
	// core := &coreService{
	// 	bc: chain,
	// 	ap: ap,
	// 	broadcastHandler: func(_ context.Context, _ uint32, _ proto.Message) error {
	// 		broadcastHandlerCount++
	// 		return nil
	// 	},
	// }
	// ge := genesis.Default
	// ge.MidwayBlockHeight = 10
	// chain.EXPECT().Genesis().Return(ge).Times(2)
	// svr := NewGRPCServer(core, 141014)
	// chain.EXPECT().ChainID().Return(uint32(1)).Times(2)
	// chain.EXPECT().TipHeight().Return(uint64(4)).Times(2)
	// ap.EXPECT().Add(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	// for i, test := range sendActionTests {
	// 	request := &iotexapi.SendActionRequest{Action: test.actionPb}
	// 	res, err := svr.SendAction(context.Background(), request)
	// 	require.NoError(err)
	// 	require.Equal(i+1, broadcastHandlerCount)
	// 	require.Equal(test.actionHash, res.ActionHash)
	// }
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
	grpcSvr := NewGRPCServer(core, 14014)
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
