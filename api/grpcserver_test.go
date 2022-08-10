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
	apitypes "github.com/iotexproject/iotex-core/api/types"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_apicoreservice"
)

func TestGrpcServer_GetAccount(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)

	t.Run("get acccount", func(t *testing.T) {
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
	})

	t.Run("failed to get account", func(t *testing.T) {
		expectedErr := errors.New("failed to get account")
		request := &iotexapi.GetAccountRequest{
			Address: "io1d4c5lp4ea4754wy439g2t99ue7wryu5r2lslh2",
		}

		core.EXPECT().Account(gomock.Any()).Return(nil, nil, expectedErr)

		_, err := grpcSvr.GetAccount(context.Background(), request)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("invalid address", func(t *testing.T) {
		expectedErr := errors.New("invalid address")
		request := &iotexapi.GetAccountRequest{
			Address: "9254d943485d0fb859ff63c5581acc44f00fc2110343ac0445b99dfe39a6f1a5",
		}

		_, err := grpcSvr.GetAccount(context.Background(), request)
		require.Contains(err.Error(), expectedErr.Error())
	})
}

func TestGrpcServer_GetActions(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)

	t.Run("get actions by address tests", func(t *testing.T) {
		for _, test := range _getActionsByAddressTests {
			actInfo := &iotexapi.ActionInfo{
				Index:   0,
				ActHash: "test",
			}
			response := []*iotexapi.ActionInfo{}
			for i := 1; i <= test.numActions; i++ {
				response = append(response, actInfo)
			}

			requests := []struct {
				req  *iotexapi.GetActionsRequest
				call func()
			}{
				{
					req: &iotexapi.GetActionsRequest{
						Lookup: &iotexapi.GetActionsRequest_ByAddr{
							ByAddr: &iotexapi.GetActionsByAddressRequest{
								Address: test.address,
								Start:   test.start,
								Count:   test.count,
							},
						},
					},
					call: func() {
						core.EXPECT().ActionsByAddress(gomock.Any(), gomock.Any(), gomock.Any()).Return(response, nil)
					},
				},
				{
					req: &iotexapi.GetActionsRequest{
						Lookup: &iotexapi.GetActionsRequest_UnconfirmedByAddr{
							UnconfirmedByAddr: &iotexapi.GetUnconfirmedActionsByAddressRequest{
								Address: test.address,
								Start:   test.start,
								Count:   test.count,
							},
						},
					},
					call: func() {
						core.EXPECT().UnconfirmedActionsByAddress(gomock.Any(), gomock.Any(), gomock.Any()).Return(response, nil)
					},
				},
				{
					req: &iotexapi.GetActionsRequest{
						Lookup: &iotexapi.GetActionsRequest_ByIndex{
							ByIndex: &iotexapi.GetActionsByIndexRequest{
								Start: test.start,
								Count: test.count,
							},
						},
					},
					call: func() {
						core.EXPECT().Actions(gomock.Any(), gomock.Any()).Return(response, nil)
					},
				},
			}

			for _, request := range requests {
				request.call()
				res, err := grpcSvr.GetActions(context.Background(), request.req)
				require.NoError(err)
				require.Equal(uint64(test.numActions), res.Total)
			}
		}
	})

	t.Run("get actions by hash", func(t *testing.T) {
		for _, test := range _getActionTests {
			response := &iotexapi.ActionInfo{
				Index:     0,
				ActHash:   "test",
				BlkHeight: test.blkNumber,
			}
			request := &iotexapi.GetActionsRequest{
				Lookup: &iotexapi.GetActionsRequest_ByHash{
					ByHash: &iotexapi.GetActionByHashRequest{
						ActionHash:   test.in,
						CheckPending: test.checkPending,
					},
				},
			}

			core.EXPECT().Action(gomock.Any(), gomock.Any()).Return(response, nil)

			res, err := grpcSvr.GetActions(context.Background(), request)
			require.NoError(err)
			require.Len(res.ActionInfo, 1)
			require.Equal(test.blkNumber, res.ActionInfo[0].BlkHeight)
		}
	})

	t.Run("get actions by block test", func(t *testing.T) {
		addr1 := identityset.Address(28).String()
		priKey1 := identityset.PrivateKey(28)

		tsf1, err := action.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
		require.NoError(err)

		for _, test := range _getActionsByBlockTests {
			gasConsumed, ok := new(big.Int).SetString(test.firstTxGas, 10)
			if !ok {
				gasConsumed = big.NewInt(0)
			}

			response := &apitypes.BlockWithReceipts{
				Block:    &block.Block{},
				Receipts: []*action.Receipt{},
			}
			for i := 1; i <= test.numActions; i++ {
				response.Block.Actions = append(response.Block.Actions, tsf1)
				response.Receipts = append(response.Receipts, &action.Receipt{
					BlockHeight: test.blkHeight,
					GasConsumed: gasConsumed.Uint64(),
				})
			}

			request := &iotexapi.GetActionsRequest{
				Lookup: &iotexapi.GetActionsRequest_ByBlk{
					ByBlk: &iotexapi.GetActionsByBlockRequest{
						BlkHash: _blkHash[test.blkHeight],
						Start:   test.start,
						Count:   test.count,
					},
				},
			}

			core.EXPECT().BlockByHash(gomock.Any()).Return(response, nil)

			res, err := grpcSvr.GetActions(context.Background(), request)
			require.NoError(err)
			require.Equal(test.numActions-int(test.start), int(res.Total))
		}
	})

	t.Run("invalid GetActionsRequest type", func(t *testing.T) {
		expectedErr := errors.New("invalid GetActionsRequest type")
		request := &iotexapi.GetActionsRequest{}

		_, err := grpcSvr.GetActions(context.Background(), request)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to get actions", func(t *testing.T) {
		expectedErr := errors.New("failed to get actions")
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByHash{
				ByHash: &iotexapi.GetActionByHashRequest{
					ActionHash:   hex.EncodeToString(_transferHash1[:]),
					CheckPending: false,
				},
			},
		}

		core.EXPECT().Action(gomock.Any(), gomock.Any()).Return(nil, expectedErr)

		_, err := grpcSvr.GetActions(context.Background(), request)
		require.Contains(err.Error(), expectedErr.Error())
	})
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
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)

	core.EXPECT().ServerMeta().Return("packageVersion", "packageCommitID", "gitStatus", "goVersion", "buildTime")
	res, err := grpcSvr.GetServerMeta(context.Background(), &iotexapi.GetServerMetaRequest{})
	require.NoError(err)
	require.Equal("packageVersion", res.ServerMeta.PackageVersion)
	require.Equal("packageCommitID", res.ServerMeta.PackageCommitID)
	require.Equal("gitStatus", res.ServerMeta.GitStatus)
	require.Equal("goVersion", res.ServerMeta.GoVersion)
	require.Equal("buildTime", res.ServerMeta.BuildTime)
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
