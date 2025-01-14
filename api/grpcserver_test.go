// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/pkg/version"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	mock_apitypes "github.com/iotexproject/iotex-core/v2/test/mock/mock_apiresponder"
)

func TestGrpcServer_GetAccount(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
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
	core := NewMockCoreService(ctrl)
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
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)

	errStr := "get block metas mock test error"
	reqIndex := &iotexapi.GetBlockMetasRequest{
		Lookup: &iotexapi.GetBlockMetasRequest_ByIndex{
			ByIndex: &iotexapi.GetBlockMetasByIndexRequest{},
		},
	}
	reqHash := &iotexapi.GetBlockMetasRequest{
		Lookup: &iotexapi.GetBlockMetasRequest_ByHash{
			ByHash: &iotexapi.GetBlockMetaByHashRequest{},
		},
	}
	ret := &apitypes.BlockWithReceipts{
		Block: &block.Block{},
		Receipts: []*action.Receipt{
			{},
		},
	}
	rets := []*apitypes.BlockWithReceipts{ret}

	t.Run("GetBlockMetasInvalidType", func(t *testing.T) {
		_, err := grpcSvr.GetBlockMetas(context.Background(), &iotexapi.GetBlockMetasRequest{})
		require.Contains(err.Error(), "invalid GetBlockMetasRequest type")
	})
	t.Run("GetBlockMetasByIndexFailed", func(t *testing.T) {
		core.EXPECT().BlockByHeightRange(gomock.Any(), gomock.Any()).Return(nil, errors.New(errStr))
		_, err := grpcSvr.GetBlockMetas(context.Background(), reqIndex)
		require.Contains(err.Error(), errStr)
	})
	t.Run("GetBlockMetasByIndexSuccess", func(t *testing.T) {
		core.EXPECT().BlockByHeightRange(gomock.Any(), gomock.Any()).Return(rets, nil)
		res, err := grpcSvr.GetBlockMetas(context.Background(), reqIndex)
		require.NoError(err)
		require.Equal(res.Total, uint64(1))
	})
	t.Run("GetBlockMetasByHashFailed", func(t *testing.T) {
		core.EXPECT().BlockByHash(gomock.Any()).Return(nil, errors.New(errStr))
		_, err := grpcSvr.GetBlockMetas(context.Background(), reqHash)
		require.Contains(err.Error(), errStr)
	})
	t.Run("GetBlockMetasByHashSuccess", func(t *testing.T) {
		core.EXPECT().BlockByHash(gomock.Any()).Return(ret, nil)
		res, err := grpcSvr.GetBlockMetas(context.Background(), reqHash)
		require.NoError(err)
		require.Equal(res.Total, uint64(1))
	})
}

func TestGrpcServer_GetChainMeta(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)
	chainMeta := &iotextypes.ChainMeta{
		Height:   1000,
		ChainID:  1,
		Epoch:    &iotextypes.EpochData{Num: 7000},
		TpsFloat: 11.22,
	}
	syncStatus := "sync ok"
	core.EXPECT().ChainMeta().Return(chainMeta, syncStatus, nil)
	request := &iotexapi.GetChainMetaRequest{}
	res, err := grpcSvr.GetChainMeta(context.Background(), request)
	require.NoError(err)
	require.Equal(chainMeta, res.ChainMeta)
	require.Equal(syncStatus, res.SyncStage)
}

func TestGrpcServer_SendAction(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)

	for _, test := range _sendActionTests {
		core.EXPECT().SendAction(context.Background(), test.actionPb).Return(test.actionHash, nil)
		request := &iotexapi.SendActionRequest{Action: test.actionPb}
		res, err := grpcSvr.SendAction(context.Background(), request)
		require.NoError(err)
		require.Equal(test.actionHash, res.ActionHash)
	}
}

func TestGrpcServer_StreamBlocks(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)

	t.Run("addResponder failed", func(t *testing.T) {
		listener := mock_apitypes.NewMockListener(ctrl)
		listener.EXPECT().AddResponder(gomock.Any()).Return("", errors.New("mock test"))
		core.EXPECT().ChainListener().Return(listener)
		err := grpcSvr.StreamBlocks(&iotexapi.StreamBlocksRequest{}, nil)
		require.Contains(err.Error(), "mock test")
	})

	t.Run("success", func(t *testing.T) {
		listener := mock_apitypes.NewMockListener(ctrl)
		listener.EXPECT().AddResponder(gomock.Any()).DoAndReturn(func(g *gRPCBlockListener) (string, error) {
			go func() {
				g.errChan <- nil
			}()
			return "", nil
		})
		listener.EXPECT().RemoveResponder(gomock.Any()).DoAndReturn(func(string) (bool, error) {
			return true, nil
		})
		core.EXPECT().ChainListener().Return(listener)
		err := grpcSvr.StreamBlocks(&iotexapi.StreamBlocksRequest{}, nil)
		require.NoError(err)
	})
}

func TestGrpcServer_StreamLogs(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)

	t.Run("StreamLogsEmptyFilter", func(t *testing.T) {
		err := grpcSvr.StreamLogs(&iotexapi.StreamLogsRequest{}, nil)
		require.Contains(err.Error(), "empty filter")
	})
	t.Run("StreamLogsAddResponderFailed", func(t *testing.T) {
		listener := mock_apitypes.NewMockListener(ctrl)
		listener.EXPECT().AddResponder(gomock.Any()).Return("", errors.New("mock test"))
		core.EXPECT().ChainListener().Return(listener)
		err := grpcSvr.StreamLogs(&iotexapi.StreamLogsRequest{Filter: &iotexapi.LogsFilter{}}, nil)
		require.Contains(err.Error(), "mock test")
	})
	t.Run("StreamLogsSuccess", func(t *testing.T) {
		listener := mock_apitypes.NewMockListener(ctrl)
		listener.EXPECT().AddResponder(gomock.Any()).DoAndReturn(func(g *gRPCLogListener) (string, error) {
			go func() {
				g.errChan <- nil
			}()
			return "", nil
		})
		listener.EXPECT().RemoveResponder(gomock.Any()).DoAndReturn(func(string) (bool, error) {
			return true, nil
		})
		core.EXPECT().ChainListener().Return(listener)
		err := grpcSvr.StreamLogs(&iotexapi.StreamLogsRequest{Filter: &iotexapi.LogsFilter{}}, nil)
		require.NoError(err)
	})
}

func TestGrpcServer_GetReceiptByAction(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)
	receipt := &action.Receipt{
		Status:          1,
		BlockHeight:     1,
		ActionHash:      hash.BytesToHash256([]byte("test")),
		GasConsumed:     1,
		ContractAddress: "test",
		TxIndex:         1,
	}

	t.Run("get receipt by action", func(t *testing.T) {
		core.EXPECT().ReceiptByActionHash(gomock.Any()).Return(receipt, nil)
		core.EXPECT().BlockHashByBlockHeight(gomock.Any()).Return(hash.ZeroHash256, nil)

		res, err := grpcSvr.GetReceiptByAction(context.Background(), &iotexapi.GetReceiptByActionRequest{})
		require.NoError(err)
		require.Equal(receipt.Status, res.ReceiptInfo.Receipt.Status)
		require.Equal(receipt.BlockHeight, res.ReceiptInfo.Receipt.BlkHeight)
		require.Equal(receipt.ActionHash[:], res.ReceiptInfo.Receipt.ActHash)
		require.Equal(receipt.GasConsumed, res.ReceiptInfo.Receipt.GasConsumed)
		require.Equal(receipt.ContractAddress, res.ReceiptInfo.Receipt.ContractAddress)
		require.Equal(receipt.TxIndex, res.ReceiptInfo.Receipt.TxIndex)
		require.Equal(hex.EncodeToString(hash.ZeroHash256[:]), res.ReceiptInfo.BlkHash)
	})

	t.Run("failed to get block hash by block height", func(t *testing.T) {
		expectedErr := errors.New("failed to get block hash by block height")
		core.EXPECT().ReceiptByActionHash(gomock.Any()).Return(receipt, nil)
		core.EXPECT().BlockHashByBlockHeight(gomock.Any()).Return(hash.ZeroHash256, expectedErr)

		_, err := grpcSvr.GetReceiptByAction(context.Background(), &iotexapi.GetReceiptByActionRequest{})
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to get reciept by action hash", func(t *testing.T) {
		expectedErr := errors.New("failed to get reciept by action hash")
		core.EXPECT().ReceiptByActionHash(gomock.Any()).Return(receipt, expectedErr)

		_, err := grpcSvr.GetReceiptByAction(context.Background(), &iotexapi.GetReceiptByActionRequest{})
		require.Contains(err.Error(), expectedErr.Error())
	})
}

func TestGrpcServer_GetServerMeta(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
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
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)
	response := &iotextypes.Receipt{
		ActHash:     []byte("08b0066e10b5607e47159c2cf7ba36e36d0c980f5108dfca0ec20547a7adace4"),
		GasConsumed: 10100,
	}

	t.Run("read contract", func(t *testing.T) {
		request := &iotexapi.ReadContractRequest{
			Execution: &iotextypes.Execution{
				Data: _executionHash1[:],
			},
			CallerAddress: identityset.Address(0).String(),
			GasLimit:      10100,
		}
		core.EXPECT().ReadContract(gomock.Any(), gomock.Any(), gomock.Any()).Return("", response, nil)

		res, err := grpcSvr.ReadContract(context.Background(), request)
		require.NoError(err)
		require.Equal([]byte("08b0066e10b5607e47159c2cf7ba36e36d0c980f5108dfca0ec20547a7adace4"), res.Receipt.ActHash)
		require.Equal(10100, int(res.Receipt.GasConsumed))
	})

	t.Run("read contract from empty address", func(t *testing.T) {
		request := &iotexapi.ReadContractRequest{
			Execution: &iotextypes.Execution{
				Data: _executionHash1[:],
			},
			CallerAddress: "",
			GasLimit:      10100,
		}
		core.EXPECT().ReadContract(gomock.Any(), gomock.Any(), gomock.Any()).Return("", response, nil)

		res, err := grpcSvr.ReadContract(context.Background(), request)
		require.NoError(err)
		require.Equal([]byte("08b0066e10b5607e47159c2cf7ba36e36d0c980f5108dfca0ec20547a7adace4"), res.Receipt.ActHash)
		require.Equal(10100, int(res.Receipt.GasConsumed))
	})

	t.Run("failed to read contract", func(t *testing.T) {
		expectedErr := errors.New("failed to read contract")
		request := &iotexapi.ReadContractRequest{
			Execution: &iotextypes.Execution{
				Data: _executionHash1[:],
			},
			CallerAddress: "",
			GasLimit:      10100,
		}
		core.EXPECT().ReadContract(gomock.Any(), gomock.Any(), gomock.Any()).Return("", nil, expectedErr)

		_, err := grpcSvr.ReadContract(context.Background(), request)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to load execution", func(t *testing.T) {
		expectedErr := errors.New("empty action proto to load")
		request := &iotexapi.ReadContractRequest{
			CallerAddress: "",
			GasLimit:      10100,
		}

		_, err := grpcSvr.ReadContract(context.Background(), request)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("invalid caller address", func(t *testing.T) {
		expectedErr := errors.New("invalid address")
		request := &iotexapi.ReadContractRequest{
			Execution: &iotextypes.Execution{
				Data: _executionHash1[:],
			},
			CallerAddress: "9254d943485d0fb859ff63c5581acc44f00fc2110343ac0445b99dfe39a6f1a5",
			GasLimit:      10100,
		}

		_, err := grpcSvr.ReadContract(context.Background(), request)
		require.Contains(err.Error(), expectedErr.Error())
	})
}

func TestGrpcServer_SuggestGasPrice(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
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
	core := NewMockCoreService(ctrl)
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
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)
	request := &iotexapi.EstimateActionGasConsumptionRequest{
		CallerAddress: identityset.Address(0).String(),
	}

	t.Run("Execution is not nil", func(t *testing.T) {
		core.EXPECT().EstimateExecutionGasConsumption(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(10100), nil, nil)
		request.Action = &iotexapi.EstimateActionGasConsumptionRequest_Execution{
			Execution: &iotextypes.Execution{
				Data: _executionHash1[:],
			},
		}
		res, err := grpcSvr.EstimateActionGasConsumption(context.Background(), request)
		require.NoError(err)
		require.Equal(uint64(10100), res.Gas)
	})

	core.EXPECT().EstimateGasForNonExecution(gomock.Any()).Return(uint64(10100), nil).Times(10)

	t.Run("Transfer is not nil", func(t *testing.T) {
		request.Action = &iotexapi.EstimateActionGasConsumptionRequest_Transfer{
			Transfer: &iotextypes.Transfer{
				Amount:    "1000",
				Recipient: identityset.Address(29).String(),
				Payload:   []byte("1234567890"),
			},
		}
		res, err := grpcSvr.EstimateActionGasConsumption(context.Background(), request)
		require.NoError(err)
		require.Equal(uint64(10100), res.Gas)
	})

	t.Run("StakeCreate is not nil", func(t *testing.T) {
		request.Action = &iotexapi.EstimateActionGasConsumptionRequest_StakeCreate{
			StakeCreate: &iotextypes.StakeCreate{
				CandidateName:  "Candidate",
				StakedAmount:   "1000",
				StakedDuration: uint32(111),
				Payload:        []byte("1234567890"),
			},
		}
		res, err := grpcSvr.EstimateActionGasConsumption(context.Background(), request)
		require.NoError(err)
		require.Equal(uint64(10100), res.Gas)
	})

	t.Run("StakeUnstake is not nil", func(t *testing.T) {
		request.Action = &iotexapi.EstimateActionGasConsumptionRequest_StakeUnstake{
			StakeUnstake: &iotextypes.StakeReclaim{
				BucketIndex: uint64(111),
				Payload:     []byte("1234567890"),
			},
		}
		res, err := grpcSvr.EstimateActionGasConsumption(context.Background(), request)
		require.NoError(err)
		require.Equal(uint64(10100), res.Gas)
	})

	t.Run("StakeWithdraw is not nil", func(t *testing.T) {
		request.Action = &iotexapi.EstimateActionGasConsumptionRequest_StakeWithdraw{
			StakeWithdraw: &iotextypes.StakeReclaim{
				BucketIndex: uint64(222),
				Payload:     []byte("abc1234567890"),
			},
		}
		res, err := grpcSvr.EstimateActionGasConsumption(context.Background(), request)
		require.NoError(err)
		require.Equal(uint64(10100), res.Gas)
	})

	t.Run("StakeAddDeposit is not nil", func(t *testing.T) {
		request.Action = &iotexapi.EstimateActionGasConsumptionRequest_StakeAddDeposit{
			StakeAddDeposit: &iotextypes.StakeAddDeposit{
				Amount:      "1000",
				BucketIndex: uint64(333),
				Payload:     []byte("abc1234567890"),
			},
		}
		res, err := grpcSvr.EstimateActionGasConsumption(context.Background(), request)
		require.NoError(err)
		require.Equal(uint64(10100), res.Gas)
	})

	t.Run("StakeRestake is not nil", func(t *testing.T) {
		request.Action = &iotexapi.EstimateActionGasConsumptionRequest_StakeRestake{
			StakeRestake: &iotextypes.StakeRestake{
				BucketIndex:    uint64(444),
				StakedDuration: uint32(111),
				Payload:        []byte("abc1234567890"),
			},
		}
		res, err := grpcSvr.EstimateActionGasConsumption(context.Background(), request)
		require.NoError(err)
		require.Equal(uint64(10100), res.Gas)
	})

	t.Run("StakeChangeCandidate is not nil", func(t *testing.T) {
		request.Action = &iotexapi.EstimateActionGasConsumptionRequest_StakeChangeCandidate{
			StakeChangeCandidate: &iotextypes.StakeChangeCandidate{
				CandidateName: "Candidate",
				BucketIndex:   uint64(555),
				Payload:       []byte("abc1234567890"),
			},
		}
		res, err := grpcSvr.EstimateActionGasConsumption(context.Background(), request)
		require.NoError(err)
		require.Equal(uint64(10100), res.Gas)
	})

	t.Run("StakeTransferOwnership is not nil", func(t *testing.T) {
		request.Action = &iotexapi.EstimateActionGasConsumptionRequest_StakeTransferOwnership{
			StakeTransferOwnership: &iotextypes.StakeTransferOwnership{
				BucketIndex:  uint64(666),
				VoterAddress: identityset.Address(29).String(),
				Payload:      []byte("1234567890"),
			},
		}
		res, err := grpcSvr.EstimateActionGasConsumption(context.Background(), request)
		require.NoError(err)
		require.Equal(uint64(10100), res.Gas)
	})

	t.Run("CandidateRegister is not nil", func(t *testing.T) {
		request.Action = &iotexapi.EstimateActionGasConsumptionRequest_CandidateRegister{
			CandidateRegister: &iotextypes.CandidateRegister{
				Candidate: &iotextypes.CandidateBasicInfo{
					Name:            "candidate",
					OperatorAddress: identityset.Address(29).String(),
					RewardAddress:   identityset.Address(28).String(),
				},
				StakedAmount:   "1000",
				StakedDuration: uint32(111),
				OwnerAddress:   identityset.Address(29).String(),
				Payload:        []byte("abc1234567890"),
			},
		}
		res, err := grpcSvr.EstimateActionGasConsumption(context.Background(), request)
		require.NoError(err)
		require.Equal(uint64(10100), res.Gas)
	})

	t.Run("CandidateUpdate is not nil", func(t *testing.T) {
		request.Action = &iotexapi.EstimateActionGasConsumptionRequest_CandidateUpdate{
			CandidateUpdate: &iotextypes.CandidateBasicInfo{
				Name:            "candidate",
				OperatorAddress: identityset.Address(29).String(),
				RewardAddress:   identityset.Address(28).String(),
			},
		}
		res, err := grpcSvr.EstimateActionGasConsumption(context.Background(), request)
		require.NoError(err)
		require.Equal(uint64(10100), res.Gas)
	})
}

func TestGrpcServer_ReadState(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)
	core.EXPECT().ReadState(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&iotexapi.ReadStateResponse{
		Data: []byte("10100"),
	}, nil)
	resp, err := grpcSvr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
		MethodName: []byte("UnclaimedBalance"),
	})
	require.NoError(err)
	require.Equal([]byte("10100"), resp.Data)
}

func TestGrpcServer_GetEpochMeta(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)
	epochData := &iotextypes.EpochData{Num: 7000}
	blockProducersInfo := []*iotexapi.BlockProducerInfo{{Production: 8000}}
	core.EXPECT().EpochMeta(gomock.Any()).Return(epochData, uint64(11), blockProducersInfo, nil)
	resp, err := grpcSvr.GetEpochMeta(context.Background(), &iotexapi.GetEpochMetaRequest{
		EpochNumber: uint64(11),
	})
	require.NoError(err)
	require.Equal(epochData, resp.EpochData)
	require.Equal(uint64(11), resp.TotalBlocks)
	require.Equal(blockProducersInfo, resp.BlockProducersInfo)
}

func TestGrpcServer_GetRawBlocks(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)

	blocks := []*iotexapi.BlockInfo{
		{
			Block: &iotextypes.Block{
				Body: &iotextypes.BlockBody{
					Actions: []*iotextypes.Action{
						{
							Core: &iotextypes.ActionCore{
								Version:  1,
								Nonce:    2,
								GasLimit: 3,
								GasPrice: "4",
							},
						},
					},
				},
			},
			Receipts: []*iotextypes.Receipt{
				{
					Status:          1,
					BlkHeight:       1,
					ActHash:         []byte("02ae2a956d21e8d481c3a69e146633470cf625ec"),
					GasConsumed:     1,
					ContractAddress: "test",
					Logs:            []*iotextypes.Log{},
				},
			},
		},
	}
	core.EXPECT().RawBlocks(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(blocks, nil)
	resp, err := grpcSvr.GetRawBlocks(context.Background(), &iotexapi.GetRawBlocksRequest{
		StartHeight: 55,
		Count:       1000,
	})
	require.NoError(err)
	require.Equal(blocks, resp.Blocks)
}

func TestGrpcServer_GetLogs(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)
	request := &iotexapi.GetLogsRequest{
		Filter: &iotexapi.LogsFilter{
			Address: []string{},
			Topics:  []*iotexapi.Topics{},
		},
	}
	logs := []*action.Log{
		{
			Address:     "_topic1",
			BlockHeight: 1,
			Topics: []hash.Hash256{
				hash.Hash256b([]byte("_topic1")),
				hash.Hash256b([]byte("_topic11")),
			},
		},
		{
			Address:     "_topic2",
			BlockHeight: 2,
			Topics: []hash.Hash256{
				hash.Hash256b([]byte("_topic2")),
				hash.Hash256b([]byte("_topic22")),
			},
		},
	}

	t.Run("by block", func(t *testing.T) {
		core.EXPECT().LogsInBlockByHash(gomock.Any(), gomock.Any()).Return(logs, nil)
		request.Lookup = &iotexapi.GetLogsRequest_ByBlock{
			ByBlock: &iotexapi.GetLogsByBlock{
				BlockHash: []byte("02ae2a956d21e8d481c3a69e146633470cf625ec"),
			},
		}
		res, err := grpcSvr.GetLogs(context.Background(), request)
		require.NoError(err)
		require.Len(res.Logs, 2)
		blkHash := hash.BytesToHash256(request.GetByBlock().BlockHash)
		require.Equal(blkHash[:], res.Logs[0].BlkHash)
		require.Equal(logs[0].Address, res.Logs[0].ContractAddress)
		require.Equal(logs[0].BlockHeight, res.Logs[0].BlkHeight)
		require.Len(res.Logs[0].Topics, 2)
		require.Equal(logs[0].Topics[0][:], res.Logs[0].Topics[0])
		require.Equal(logs[0].Topics[1][:], res.Logs[0].Topics[1])
		require.Equal(blkHash[:], res.Logs[1].BlkHash)
		require.Equal(logs[1].Address, res.Logs[1].ContractAddress)
		require.Equal(logs[1].BlockHeight, res.Logs[1].BlkHeight)
		require.Len(res.Logs[1].Topics, 2)
		require.Equal(logs[1].Topics[0][:], res.Logs[1].Topics[0])
		require.Equal(logs[1].Topics[1][:], res.Logs[1].Topics[1])
	})

	t.Run("by range", func(t *testing.T) {
		hashes := []hash.Hash256{
			hash.BytesToHash256([]byte("02ae2a956d21e8d481c3a69e146633470cf625ec")),
			hash.BytesToHash256([]byte("956d21e8d481c3a6901fc246633470cf62ae2ae1")),
		}
		core.EXPECT().LogsInRange(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(logs, hashes, nil)
		request.Lookup = &iotexapi.GetLogsRequest_ByRange{
			ByRange: &iotexapi.GetLogsByRange{
				FromBlock: 1,
				ToBlock:   100,
			},
		}
		res, err := grpcSvr.GetLogs(context.Background(), request)
		require.NoError(err)
		require.Len(res.Logs, 2)
		require.Equal(hashes[0][:], res.Logs[0].BlkHash)
		require.Equal(logs[0].Address, res.Logs[0].ContractAddress)
		require.Equal(logs[0].BlockHeight, res.Logs[0].BlkHeight)
		require.Len(res.Logs[0].Topics, 2)
		require.Equal(logs[0].Topics[0][:], res.Logs[0].Topics[0])
		require.Equal(logs[0].Topics[1][:], res.Logs[0].Topics[1])
		require.Equal(hashes[1][:], res.Logs[1].BlkHash)
		require.Equal(logs[1].Address, res.Logs[1].ContractAddress)
		require.Equal(logs[1].BlockHeight, res.Logs[1].BlkHeight)
		require.Len(res.Logs[1].Topics, 2)
		require.Equal(logs[1].Topics[0][:], res.Logs[1].Topics[0])
		require.Equal(logs[1].Topics[1][:], res.Logs[1].Topics[1])
	})
}

func TestGrpcServer_GetElectionBuckets(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)

	buckets := []*iotextypes.ElectionBucket{
		{
			Voter:     []byte("_voter"),
			Candidate: []byte("_candidate"),
		},
	}
	core.EXPECT().ElectionBuckets(gomock.Any()).Return(buckets, nil)
	resp, err := grpcSvr.GetElectionBuckets(context.Background(), &iotexapi.GetElectionBucketsRequest{
		EpochNum: 11,
	})
	require.NoError(err)
	require.Equal(buckets, resp.Buckets)
}

func TestGrpcServer_GetTransactionLogByActionHash(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)

	txLog := &iotextypes.TransactionLog{
		ActionHash:      []byte("_testAction"),
		NumTransactions: 100,
	}
	core.EXPECT().TransactionLogByActionHash(gomock.Any()).Return(txLog, nil)
	resp, err := grpcSvr.GetTransactionLogByActionHash(context.Background(), &iotexapi.GetTransactionLogByActionHashRequest{
		ActionHash: "_actionHash",
	})
	require.NoError(err)
	require.Equal(txLog, resp.TransactionLog)
}

func TestGrpcServer_GetTransactionLogByBlockHeight(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)

	blockIdentifier := &iotextypes.BlockIdentifier{
		Height: 10100,
	}
	txLog := &iotextypes.TransactionLogs{
		Logs: []*iotextypes.TransactionLog{
			{
				ActionHash:      []byte("_testAction"),
				NumTransactions: 100,
			},
		},
	}
	core.EXPECT().TransactionLogByBlockHeight(gomock.Any()).Return(blockIdentifier, txLog, nil)
	resp, err := grpcSvr.GetTransactionLogByBlockHeight(context.Background(), &iotexapi.GetTransactionLogByBlockHeightRequest{
		BlockHeight: 10101,
	})
	require.NoError(err)
	require.Equal(blockIdentifier, resp.BlockIdentifier)
	require.Equal(txLog, resp.TransactionLogs)
}

func TestGrpcServer_GetActPoolActions(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)

	addr1 := identityset.Address(28).String()
	priKey1 := identityset.PrivateKey(29)
	tsf1, err := action.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(10), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	tsf2, err := action.SignedTransfer(addr1, priKey1, uint64(2), big.NewInt(20), []byte{}, uint64(100000), big.NewInt(0))
	require.NoError(err)
	acts := []*action.SealedEnvelope{tsf1, tsf2}
	core.EXPECT().ActionsInActPool(gomock.Any()).Return(acts, nil)
	resp, err := grpcSvr.GetActPoolActions(context.Background(), &iotexapi.GetActPoolActionsRequest{
		ActionHashes: []string{"_actionHash"},
	})
	require.NoError(err)
	require.Equal(len(acts), len(resp.Actions))
	require.Equal(acts[0].Signature(), resp.Actions[0].Signature)
	require.Equal(acts[1].Signature(), resp.Actions[1].Signature)
	require.Equal(acts[0].SrcPubkey().Bytes(), resp.Actions[0].SenderPubKey)
	require.Equal(acts[1].SrcPubkey().Bytes(), resp.Actions[1].SenderPubKey)
}

func TestGrpcServer_ReadContractStorage(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)

	core.EXPECT().ReadContractStorage(gomock.Any(), gomock.Any(), gomock.Any()).Return([]byte("_data"), nil)
	resp, err := grpcSvr.ReadContractStorage(context.Background(), &iotexapi.ReadContractStorageRequest{
		Contract: "io1d4c5lp4ea4754wy439g2t99ue7wryu5r2lslh2",
		Key:      []byte("_key"),
	})
	require.NoError(err)
	require.Equal([]byte("_data"), resp.Data)
}

func TestGrpcServer_TraceTransactionStructLogs(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)

	core.EXPECT().TraceTransaction(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, logger.NewStructLogger(nil), nil)
	resp, err := grpcSvr.TraceTransactionStructLogs(context.Background(), &iotexapi.TraceTransactionStructLogsRequest{
		ActionHash: "_actionHash",
	})
	require.NoError(err)
	require.Equal(0, len(resp.StructLogs))
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
