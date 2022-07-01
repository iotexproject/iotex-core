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
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_apicoreservice"
)

func TestGrpcServer_GetAccount(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	grpcSvr := NewGRPCServer(core, testutil.RandomPort())

	for _, test := range _getAccountTests {
		accountMeta := &iotextypes.AccountMeta{
			Address: test.address,
		}
		blockIdentifier := &iotextypes.BlockIdentifier{}
		request := &iotexapi.GetAccountRequest{
			Address: test.address,
		}

		core.EXPECT().Account(gomock.Any()).Return(accountMeta, blockIdentifier, nil)

		res, err := grpcSvr.GetAccount(context.Background(), request)
		require.NoError(err)
		require.Equal(test.address, res.AccountMeta.Address)
	}
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
	grpcSvr := newGRPCHandler(core)

	for _, test := range _sendActionTests {
		core.EXPECT().EVMNetworkID().Return(uint32(1))
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
	grpcSvr := newGRPCHandler(core)

	core.EXPECT().SuggestGasPrice().Return(uint64(1), nil)
	res, err := grpcSvr.SuggestGasPrice(context.Background(), &iotexapi.SuggestGasPriceRequest{})
	require.NoError(err)
	require.Equal(uint64(1), res.GasPrice)

	core.EXPECT().SuggestGasPrice().Return(uint64(0), errors.New("mock gas price error"))
	_, err = grpcSvr.SuggestGasPrice(context.Background(), &iotexapi.SuggestGasPriceRequest{})
	require.Contains(err.Error(), "mock gas price error")
}

func TestGrpcServer_EstimateGasForAction(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)

	core.EXPECT().EstimateGasForAction(gomock.Any(), gomock.Any()).Return(uint64(10000), nil)
	resp, err := grpcSvr.EstimateGasForAction(context.Background(), &iotexapi.EstimateGasForActionRequest{Action: getAction()})
	require.NoError(err)
	require.Equal(uint64(10000), resp.Gas)

	core.EXPECT().EstimateGasForAction(gomock.Any(), gomock.Any()).Return(uint64(0), action.ErrNilProto)
	_, err = grpcSvr.EstimateGasForAction(context.Background(), &iotexapi.EstimateGasForActionRequest{Action: nil})
	require.Contains(err.Error(), action.ErrNilProto.Error())
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

func getAction() (act *iotextypes.Action) {
	pubKey1 := identityset.PrivateKey(28).PublicKey()
	addr2 := identityset.Address(29).String()

	act = &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_Transfer{
				Transfer: &iotextypes.Transfer{Recipient: addr2},
			},
			Version: version.ProtocolVersion,
			Nonce:   101,
		},
		SenderPubKey: pubKey1.Bytes(),
		Signature:    action.ValidSig,
	}
	return
}

func getActionWithPayload() (act *iotextypes.Action) {
	pubKey1 := identityset.PrivateKey(28).PublicKey()
	addr2 := identityset.Address(29).String()

	act = &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_Transfer{
				Transfer: &iotextypes.Transfer{Recipient: addr2, Payload: []byte("1234567890")},
			},
			Version: version.ProtocolVersion,
			Nonce:   101,
		},
		SenderPubKey: pubKey1.Bytes(),
		Signature:    action.ValidSig,
	}
	return
}
