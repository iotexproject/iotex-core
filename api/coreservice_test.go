// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/hex"
	"math/big"
	"strconv"
	"testing"
	"time"

	. "github.com/agiledragon/gomonkey/v2"
	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-election/test/mock/mock_committee"
	"github.com/iotexproject/iotex-election/types"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/api/logfilter"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/blockchain/filedao"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/blockindex"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/server/itx/nodestats"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	mock_apitypes "github.com/iotexproject/iotex-core/v2/test/mock/mock_apiresponder"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockdao"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockindex"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blocksync"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_envelope"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_factory"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestLogsInRange(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestCoreService()
	defer cleanCallback()

	t.Run("blocks with four logs", func(t *testing.T) {
		testData := &filterObject{FromBlock: "1", ToBlock: "4"}
		filter, err := getTopicsAddress(testData.Address, testData.Topics)
		require.NoError(err)
		from, err := strconv.ParseUint(testData.FromBlock, 10, 64)
		require.NoError(err)
		to, err := strconv.ParseUint(testData.ToBlock, 10, 64)
		require.NoError(err)

		logs, hashes, err := svr.LogsInRange(logfilter.NewLogFilter(filter), from, to, uint64(0))
		require.NoError(err)
		require.Equal(4, len(logs))
		require.Equal(4, len(hashes))
	})
	t.Run("empty log", func(t *testing.T) {
		testData := &filterObject{FromBlock: "1", ToBlock: "4", Address: []string{"0x8ce313ab12bf7aed8136ab36c623ff98c8eaad34"}}
		filter, err := getTopicsAddress(testData.Address, testData.Topics)
		require.NoError(err)
		from, err := strconv.ParseUint(testData.FromBlock, 10, 64)
		require.NoError(err)
		to, err := strconv.ParseUint(testData.ToBlock, 10, 64)
		require.NoError(err)

		logs, hashes, err := svr.LogsInRange(logfilter.NewLogFilter(filter), from, to, uint64(0))
		require.NoError(err)
		require.Equal(0, len(logs))
		require.Equal(0, len(hashes))
	})
	t.Run("over 5000 pagenation size", func(t *testing.T) {
		testData := &filterObject{FromBlock: "1", ToBlock: "4"}
		filter, err := getTopicsAddress(testData.Address, testData.Topics)
		require.NoError(err)
		from, err := strconv.ParseUint(testData.FromBlock, 10, 64)
		require.NoError(err)
		to, err := strconv.ParseUint(testData.ToBlock, 10, 64)
		require.NoError(err)

		logs, hashes, err := svr.LogsInRange(logfilter.NewLogFilter(filter), from, to, uint64(5001))
		require.NoError(err)
		require.Equal(4, len(logs))
		require.Equal(4, len(hashes))
	})
	t.Run("invalid start and end height", func(t *testing.T) {
		testData := &filterObject{FromBlock: "2", ToBlock: "1"}
		filter, err := getTopicsAddress(testData.Address, testData.Topics)
		require.NoError(err)
		from, err := strconv.ParseUint(testData.FromBlock, 10, 64)
		require.NoError(err)
		to, err := strconv.ParseUint(testData.ToBlock, 10, 64)
		require.NoError(err)

		_, _, err = svr.LogsInRange(logfilter.NewLogFilter(filter), from, to, uint64(0))
		expectedErr := errors.New("invalid start or end height")
		require.Error(err)
		require.Equal(expectedErr.Error(), err.Error())
	})
	t.Run("start block > tip height", func(t *testing.T) {
		testData := &filterObject{FromBlock: "5", ToBlock: "5"}
		filter, err := getTopicsAddress(testData.Address, testData.Topics)
		require.NoError(err)
		from, err := strconv.ParseUint(testData.FromBlock, 10, 64)
		require.NoError(err)
		to, err := strconv.ParseUint(testData.ToBlock, 10, 64)
		require.NoError(err)

		_, _, err = svr.LogsInRange(logfilter.NewLogFilter(filter), from, to, uint64(0))
		expectedErr := errors.New("start block > tip height")
		require.Error(err)
		require.Equal(expectedErr.Error(), err.Error())
	})
}

func BenchmarkLogsInRange(b *testing.B) {
	svr, _, _, _, cleanCallback := setupTestCoreService()
	defer cleanCallback()

	ctrl := gomock.NewController(b)
	defer ctrl.Finish()
	blk := mock_blockindex.NewMockBloomFilterIndexer(ctrl)

	testData := &filterObject{FromBlock: "0x1"}
	filter, _ := getTopicsAddress(testData.Address, testData.Topics)
	from, _ := strconv.ParseInt(testData.FromBlock, 10, 64)
	to, _ := strconv.ParseInt(testData.ToBlock, 10, 64)

	b.Run("five workers to extract logs", func(b *testing.B) {
		blk.EXPECT().FilterBlocksInRange(logfilter.NewLogFilter(filter), uint64(from), uint64(to), 0).Return([]uint64{1, 2, 3, 4}, nil).AnyTimes()
		for i := 0; i < b.N; i++ {
			svr.LogsInRange(logfilter.NewLogFilter(filter), uint64(from), uint64(to), uint64(0))
		}
	})
}

func getTopicsAddress(addr []string, topics [][]string) (*iotexapi.LogsFilter, error) {
	var filter iotexapi.LogsFilter
	for _, ethAddr := range addr {
		ioAddr, err := ethAddrToIoAddr(ethAddr)
		if err != nil {
			return nil, err
		}
		filter.Address = append(filter.Address, ioAddr.String())
	}
	for _, tp := range topics {
		var topic [][]byte
		for _, str := range tp {
			b, err := hexToBytes(str)
			if err != nil {
				return nil, err
			}
			topic = append(topic, b)
		}
		filter.Topics = append(filter.Topics, &iotexapi.Topics{Topic: topic})
	}

	return &filter, nil
}

func setupTestCoreService() (CoreService, blockchain.Blockchain, blockdao.BlockDAO, actpool.ActPool, func()) {
	cfg := newConfig()

	// TODO (zhi): revise
	bc, dao, indexer, bfIndexer, sf, ap, registry, bfIndexFile, err := setupChain(cfg)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	// Start blockchain
	if err := bc.Start(ctx); err != nil {
		panic(err)
	}
	// Add testing blocks
	if err := addTestingBlocks(bc, ap); err != nil {
		panic(err)
	}

	opts := []Option{WithBroadcastOutbound(func(ctx context.Context, chainID uint32, msg proto.Message) error {
		return nil
	})}
	svr, err := newCoreService(cfg.api, bc, nil, sf, dao, indexer, bfIndexer, ap, registry, opts...)
	if err != nil {
		panic(err)
	}

	return svr, bc, dao, ap, func() {
		testutil.CleanupPath(bfIndexFile)
	}
}

func TestEstimateGasForAction(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svr, _, _, _, cleanCallback := setupTestCoreService()
	defer cleanCallback()

	estimatedGas, err := svr.EstimateGasForAction(context.Background(), getAction())
	require.NoError(err)
	require.Equal(uint64(10000), estimatedGas)

	estimatedGas, err = svr.EstimateGasForAction(context.Background(), getActionWithPayload())
	require.NoError(err)
	require.Equal(uint64(10000)+10*action.ExecutionDataGas, estimatedGas)

	_, err = svr.EstimateGasForAction(context.Background(), nil)
	require.Contains(err.Error(), action.ErrNilProto.Error())
}

func TestElectionBuckets(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("ElectionCommitteeIsNil", func(t *testing.T) {
		cs := &coreService{}
		_, err := cs.ElectionBuckets(uint64(0))
		require.ErrorContains(err, "Native election no supported")
	})

	var (
		committee = mock_committee.NewMockCommittee(ctrl)
		cs        = &coreService{
			electionCommittee: committee,
		}
	)

	t.Run("FailedToNativeBucketsByEpoch", func(t *testing.T) {
		committee.EXPECT().NativeBucketsByEpoch(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)
		_, err := cs.ElectionBuckets(uint64(0))
		require.ErrorContains(err, t.Name())
	})

	t.Run("ElectionBucketsSuccess", func(t *testing.T) {
		vote, err := types.NewBucket(
			time.Now().Add(-20*time.Hour),
			time.Hour*3,
			big.NewInt(9),
			[]byte("voter"),
			[]byte("candidate"),
			false,
		)
		require.NoError(err)

		committee.EXPECT().NativeBucketsByEpoch(gomock.Any()).Return([]*types.Bucket{vote}, nil).Times(1)
		re, err := cs.ElectionBuckets(uint64(0))
		require.NoError(err)
		require.Equal(1, len(re))
		require.Equal(vote.Voter(), re[0].Voter)
	})
}

func TestTransactionLogByActionHash(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("IndexerIsNil", func(t *testing.T) {
		cs := &coreService{}
		_, err := cs.TransactionLogByActionHash("")
		require.ErrorContains(err, blockindex.ErrActionIndexNA.Error())
	})

	var (
		blkDAO  = mock_blockdao.NewMockBlockDAO(ctrl)
		indexer = mock_blockindex.NewMockIndexer(ctrl)
		cs      = &coreService{
			dao:     blkDAO,
			indexer: indexer,
		}
	)

	t.Run("NotContainsTxLog", func(t *testing.T) {
		blkDAO.EXPECT().ContainsTransactionLog().Return(false).Times(1)
		_, err := cs.TransactionLogByActionHash("")
		require.ErrorContains(err, filedao.ErrNotSupported.Error())
	})

	t.Run("FailedToDecodeString", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		blkDAO.EXPECT().ContainsTransactionLog().Return(true).Times(1)

		p = p.ApplyFuncReturn(hex.DecodeString, nil, errors.New(t.Name()))

		_, err := cs.TransactionLogByActionHash("")
		require.ErrorContains(err, t.Name())
	})

	t.Run("FailedToGetActionIndex", func(t *testing.T) {
		t.Run("EqualErrNotExist", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			blkDAO.EXPECT().ContainsTransactionLog().Return(true).Times(1)
			indexer.EXPECT().GetActionIndex(gomock.Any()).Return(nil, db.ErrNotExist).Times(1)

			p = p.ApplyFuncReturn(hex.DecodeString, []byte("actHash"), nil)

			_, err := cs.TransactionLogByActionHash("")
			require.ErrorContains(err, db.ErrNotExist.Error())
		})

		t.Run("NotEqualErrNotExist", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			blkDAO.EXPECT().ContainsTransactionLog().Return(true).Times(1)
			indexer.EXPECT().GetActionIndex(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)

			p = p.ApplyFuncReturn(hex.DecodeString, []byte("actHash"), nil)

			_, err := cs.TransactionLogByActionHash("")
			require.ErrorContains(err, t.Name())
		})
	})

	t.Run("FailedToTransactionLogs", func(t *testing.T) {
		t.Run("EqualErrNotExist", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			blkDAO.EXPECT().ContainsTransactionLog().Return(true).Times(1)
			blkDAO.EXPECT().TransactionLogs(gomock.Any()).Return(nil, db.ErrNotExist).Times(1)
			indexer.EXPECT().GetActionIndex(gomock.Any()).Return(&blockindex.ActionIndex{}, nil).Times(1)

			p = p.ApplyFuncReturn(hex.DecodeString, []byte("actHash"), nil)

			_, err := cs.TransactionLogByActionHash("")
			require.ErrorContains(err, db.ErrNotExist.Error())
		})

		t.Run("NotEqualErrNotExist", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			blkDAO.EXPECT().ContainsTransactionLog().Return(true).Times(1)
			blkDAO.EXPECT().TransactionLogs(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)
			indexer.EXPECT().GetActionIndex(gomock.Any()).Return(&blockindex.ActionIndex{}, nil).Times(1)

			p = p.ApplyFuncReturn(hex.DecodeString, []byte("actHash"), nil)

			_, err := cs.TransactionLogByActionHash("")
			require.ErrorContains(err, t.Name())
		})
	})

	t.Run("NotFound", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		blkDAO.EXPECT().ContainsTransactionLog().Return(true).Times(1)
		blkDAO.EXPECT().TransactionLogs(gomock.Any()).Return(&iotextypes.TransactionLogs{}, nil).Times(1)
		indexer.EXPECT().GetActionIndex(gomock.Any()).Return(&blockindex.ActionIndex{}, nil).Times(1)

		p = p.ApplyFuncReturn(hex.DecodeString, []byte("actHash"), nil)

		_, err := cs.TransactionLogByActionHash("")
		require.ErrorContains(err, "transaction log not found for action")
	})
}

func TestTransactionLogByBlockHeight(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		blkDAO = mock_blockdao.NewMockBlockDAO(ctrl)
		cs     = &coreService{dao: blkDAO}
	)

	t.Run("NotContainsTxLog", func(t *testing.T) {
		blkDAO.EXPECT().ContainsTransactionLog().Return(false).Times(1)
		_, _, err := cs.TransactionLogByBlockHeight(uint64(0))
		require.ErrorContains(err, filedao.ErrNotSupported.Error())
	})

	t.Run("FailedToHeight", func(t *testing.T) {
		blkDAO.EXPECT().ContainsTransactionLog().Return(true).Times(1)
		blkDAO.EXPECT().Height().Return(uint64(0), errors.New(t.Name())).Times(1)

		_, _, err := cs.TransactionLogByBlockHeight(uint64(0))
		require.ErrorContains(err, t.Name())
	})

	t.Run("CheckBlockHeight", func(t *testing.T) {
		t.Run("BlockHeightLessThanOne", func(t *testing.T) {
			blkDAO.EXPECT().ContainsTransactionLog().Return(true).Times(1)
			blkDAO.EXPECT().Height().Return(uint64(0), nil).Times(1)

			_, _, err := cs.TransactionLogByBlockHeight(uint64(0))
			require.ErrorContains(err, "invalid block height")
		})

		t.Run("BlockHeightGreaterThanTip", func(t *testing.T) {
			blkDAO.EXPECT().ContainsTransactionLog().Return(true).Times(1)
			blkDAO.EXPECT().Height().Return(uint64(0), nil).Times(1)

			_, _, err := cs.TransactionLogByBlockHeight(uint64(2))
			require.ErrorContains(err, "invalid block height")
		})
	})

	t.Run("FailedToGetBlockHash", func(t *testing.T) {
		t.Run("EqualErrNotExist", func(t *testing.T) {
			blkDAO.EXPECT().ContainsTransactionLog().Return(true).Times(1)
			blkDAO.EXPECT().Height().Return(uint64(2), nil).Times(1)
			blkDAO.EXPECT().GetBlockHash(gomock.Any()).Return(hash.Hash256{}, db.ErrNotExist).Times(1)

			_, _, err := cs.TransactionLogByBlockHeight(uint64(1))
			require.ErrorContains(err, db.ErrNotExist.Error())
		})

		t.Run("NotEqualErrNotExist", func(t *testing.T) {
			blkDAO.EXPECT().ContainsTransactionLog().Return(true).Times(1)
			blkDAO.EXPECT().Height().Return(uint64(2), nil).Times(1)
			blkDAO.EXPECT().GetBlockHash(gomock.Any()).Return(hash.Hash256{}, errors.New(t.Name())).Times(1)

			_, _, err := cs.TransactionLogByBlockHeight(uint64(1))
			require.ErrorContains(err, t.Name())
		})
	})

	t.Run("FailedToTransactionLogs", func(t *testing.T) {
		t.Run("EqualErrNotExist", func(t *testing.T) {
			blkDAO.EXPECT().ContainsTransactionLog().Return(true).Times(1)
			blkDAO.EXPECT().Height().Return(uint64(2), nil).Times(1)
			blkDAO.EXPECT().GetBlockHash(gomock.Any()).Return(hash.Hash256{}, nil).Times(1)
			blkDAO.EXPECT().TransactionLogs(gomock.Any()).Return(nil, db.ErrNotExist).Times(1)

			blockIdentifier, _, err := cs.TransactionLogByBlockHeight(uint64(1))
			require.NoError(err)
			require.Equal(uint64(1), blockIdentifier.Height)
		})

		t.Run("NotEqualErrNotExist", func(t *testing.T) {
			blkDAO.EXPECT().ContainsTransactionLog().Return(true).Times(1)
			blkDAO.EXPECT().Height().Return(uint64(2), nil).Times(1)
			blkDAO.EXPECT().GetBlockHash(gomock.Any()).Return(hash.Hash256{}, nil).Times(1)
			blkDAO.EXPECT().TransactionLogs(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)

			_, _, err := cs.TransactionLogByBlockHeight(uint64(1))
			require.ErrorContains(err, t.Name())
		})
	})
}

func TestEstimateExecutionGasConsumption(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		bc = mock_blockchain.NewMockBlockchain(ctrl)
		sf = mock_factory.NewMockFactory(ctrl)
		cs = &coreService{
			bc: bc,
			sf: sf,
		}
		ctx = context.Background()
	)

	t.Run("FailedToAccountState", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		p = p.ApplyFuncReturn(accountutil.AccountStateWithHeight, nil, uint64(0), errors.New(t.Name()))

		bc.EXPECT().Genesis().Return(genesis.Genesis{}).Times(1)
		bc.EXPECT().TipHeight().Return(uint64(1)).Times(2)
		bc.EXPECT().Context(gomock.Any()).Return(ctx, nil).Times(1)
		sf.EXPECT().WorkingSet(gomock.Any()).Return(nil, nil).Times(1)
		elp := (&action.EnvelopeBuilder{}).SetAction(&action.Execution{}).Build()
		_, _, err := cs.EstimateExecutionGasConsumption(ctx, elp, &address.AddrV1{})
		require.ErrorContains(err, t.Name())
	})

	t.Run("FailedToCheckGasLimitEnough", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		p = p.ApplyFuncReturn(accountutil.AccountState, &state.Account{}, nil)
		p = p.ApplyMethodReturn(&genesis.Blockchain{}, "BlockGasLimitByHeight", uint64(0))
		p = p.ApplyPrivateMethod(
			cs,
			"isGasLimitEnough",
			func(
				context.Context,
				address.Address,
				*action.Envelope,
				...protocol.SimulateOption,
			) (bool, *action.Receipt, []byte, error) {
				return false, nil, nil, errors.New(t.Name())
			},
		)

		bc.EXPECT().Genesis().Return(genesis.Genesis{}).Times(1)
		bc.EXPECT().TipHeight().Return(uint64(0)).Times(1)
		elp := (&action.EnvelopeBuilder{}).SetAction(&action.Execution{}).Build()
		_, _, err := cs.EstimateExecutionGasConsumption(ctx, elp, &address.AddrV1{})
		require.ErrorContains(err, t.Name())
	})

	t.Run("GasLimitNotEnough", func(t *testing.T) {
		t.Run("ExecutionRevertMsgIsNil", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			receipt := &action.Receipt{}
			p = p.ApplyFuncReturn(accountutil.AccountState, &state.Account{}, nil)
			p = p.ApplyMethodReturn(&genesis.Blockchain{}, "BlockGasLimitByHeight", uint64(0))
			p = p.ApplyPrivateMethod(
				cs,
				"isGasLimitEnough",
				func(
					context.Context,
					address.Address,
					*action.Envelope,
					...protocol.SimulateOption,
				) (bool, *action.Receipt, []byte, error) {
					return false, receipt, nil, nil
				},
			)
			p = p.ApplyMethodReturn(receipt, "ExecutionRevertMsg", "")

			bc.EXPECT().Genesis().Return(genesis.Genesis{}).Times(1)
			bc.EXPECT().TipHeight().Return(uint64(0)).Times(1)
			elp := (&action.EnvelopeBuilder{}).SetAction(&action.Execution{}).Build()
			_, _, err := cs.EstimateExecutionGasConsumption(ctx, elp, &address.AddrV1{})
			require.ErrorContains(err, "execution simulation failed:")
		})

		t.Run("ExecutionRevertMsgIsNotNil", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			receipt := &action.Receipt{
				Status: uint64(iotextypes.ReceiptStatus_ErrExecutionReverted),
			}
			p = p.ApplyFuncReturn(accountutil.AccountState, &state.Account{}, nil)
			p = p.ApplyMethodReturn(&genesis.Blockchain{}, "BlockGasLimitByHeight", uint64(0))
			p = p.ApplyPrivateMethod(
				cs,
				"isGasLimitEnough",
				func(
					context.Context,
					address.Address,
					*action.Envelope,
					...protocol.SimulateOption,
				) (bool, *action.Receipt, []byte, error) {
					return false, receipt, nil, nil
				},
			)
			p = p.ApplyMethodReturn(receipt, "ExecutionRevertMsg", "TestRevertMsg")

			bc.EXPECT().Genesis().Return(genesis.Genesis{}).Times(1)
			bc.EXPECT().TipHeight().Return(uint64(0)).Times(1)
			elp := (&action.EnvelopeBuilder{}).SetAction(&action.Execution{}).Build()
			_, _, err := cs.EstimateExecutionGasConsumption(ctx, elp, &address.AddrV1{})
			require.ErrorContains(err, "execution simulation is reverted due to the reason:")
		})
	})

	t.Run("EstimateExecutionGasConsumptionSuccess", func(t *testing.T) {
		svr, _, _, _, cleanCallback := setupTestCoreService()
		defer cleanCallback()

		callAddr := identityset.Address(29)
		sc := action.NewExecution("", big.NewInt(0), []byte{})

		//gasprice is zero
		elp := (&action.EnvelopeBuilder{}).SetAction(sc).Build()
		estimatedGas, _, err := svr.EstimateExecutionGasConsumption(context.Background(), elp, callAddr)
		require.NoError(err)
		require.Equal(uint64(10000), estimatedGas)

		//gasprice no zero, should return error
		elp = (&action.EnvelopeBuilder{}).SetGasPrice(big.NewInt(100)).SetAction(sc).Build()
		estimatedGas, _, err = svr.EstimateExecutionGasConsumption(context.Background(), elp, callAddr)
		require.ErrorContains(err, "rpc error: code = Internal desc = insufficient funds for gas * price + value")
		require.Zero(estimatedGas)
	})
}

func TestTraceTransaction(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svr, bc, _, ap, cleanCallback := setupTestCoreService()
	defer cleanCallback()
	ctx := context.Background()
	tsf, err := action.SignedExecution(identityset.Address(29).String(),
		identityset.PrivateKey(29), 1, big.NewInt(0), testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64), []byte{})
	require.NoError(err)
	tsfhash, err := tsf.Hash()

	blk1Time := testutil.TimestampNow()
	require.NoError(ap.Add(ctx, tsf))
	blk, err := bc.MintNewBlock(blk1Time)
	require.NoError(err)
	require.NoError(bc.CommitBlock(blk))
	cfg := &tracers.TraceConfig{
		Config: &logger.Config{
			EnableMemory:     true,
			DisableStack:     false,
			DisableStorage:   false,
			EnableReturnData: true,
		},
	}
	retval, receipt, traces, err := svr.TraceTransaction(ctx, hex.EncodeToString(tsfhash[:]), cfg)
	require.NoError(err)
	require.Equal("0x", byteToHex(retval))
	require.Equal(uint64(1), receipt.Status)
	require.Equal(uint64(0x2710), receipt.GasConsumed)
	require.Empty(receipt.ExecutionRevertMsg())
	require.Equal(0, len(traces.(*logger.StructLogger).StructLogs()))
}

func TestTraceCall(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svr, bc, _, ap, cleanCallback := setupTestCoreService()
	defer cleanCallback()
	ctx := context.Background()
	tsf, err := action.SignedExecution(identityset.Address(29).String(),
		identityset.PrivateKey(29), 1, big.NewInt(0), testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64), []byte{})
	require.NoError(err)

	blk1Time := testutil.TimestampNow()
	require.NoError(ap.Add(ctx, tsf))
	blk, err := bc.MintNewBlock(blk1Time)
	require.NoError(err)
	require.NoError(bc.CommitBlock(blk))
	cfg := &tracers.TraceConfig{
		Config: &logger.Config{
			EnableMemory:     true,
			DisableStack:     false,
			DisableStorage:   false,
			EnableReturnData: true,
		},
	}
	retval, receipt, traces, err := svr.TraceCall(ctx,
		identityset.Address(29), blk.Height(),
		identityset.Address(29).String(),
		0, big.NewInt(0), testutil.TestGasLimit,
		[]byte{}, cfg)
	require.NoError(err)
	require.Equal("0x", byteToHex(retval))
	require.Equal(uint64(1), receipt.Status)
	require.Equal(uint64(0x2710), receipt.GasConsumed)
	require.Empty(receipt.ExecutionRevertMsg())
	require.Equal(0, len(traces.(*logger.StructLogger).StructLogs()))
}

func TestProofAndCompareReverseActions(t *testing.T) {
	sliceN := func(n uint64) (value []uint64) {
		value = make([]uint64, 0, n)
		for i := uint64(0); i < n; i++ {
			value = append(value, i)
		}
		return
	}

	// previous algorithm: commit(06d202)
	prev := func(slice []uint64, start, count uint64) (reserved []uint64) {
		size := uint64(len(slice))
		for i := start; i < size && i < start+count; i++ {
			ri := size - 1 - i
			// do other validations
			reserved = append([]uint64{slice[ri]}, reserved...)
		}
		return
	}
	// enhanced algorithm
	curr := func(slice []uint64, start, count uint64) (reserved []uint64) {
		size := uint64(len(slice))
		if start > size || count == 0 {
			return nil
		}
		end := start + count
		if end > size {
			end = size
		}
		for i := end; i > start; i-- {
			reserved = append(reserved, slice[size-i])
		}
		return
	}
	slice10 := sliceN(10)
	cases := []struct {
		name   string
		slice  []uint64
		start  uint64
		count  uint64
		expect []uint64
	}{
		{
			name:   "NoReverseDone_StartOutOfRange_EqualSliceLen",
			slice:  slice10,
			start:  10,
			count:  10,
			expect: nil,
		}, {
			name:   "NoReversedDone_StartOutOfRange_GreaterSliceLen",
			slice:  slice10,
			start:  11,
			count:  1,
			expect: nil,
		}, {
			name:   "NoReversedDone_CountIsZero",
			slice:  slice10,
			start:  9,
			count:  0,
			expect: nil,
		}, {
			name:   "StartInRangeAndEndOutOfRange",
			slice:  slice10,
			start:  5,
			count:  100,
			expect: []uint64{0, 1, 2, 3, 4},
		}, {
			name:   "StartAndEndInRangeBoth",
			slice:  slice10,
			start:  5,
			count:  3,
			expect: []uint64{2, 3, 4},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := require.New(t)
			prevExpect := prev(c.slice, c.start, c.count)
			currExpect := curr(c.slice, c.start, c.count)
			r.Equal(prevExpect, currExpect)
			r.Equal(c.expect, prevExpect)
		})
	}
}

func TestReverseActionsInBlock(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		blkDAO   = mock_blockdao.NewMockBlockDAO(ctrl)
		envelope = mock_envelope.NewMockEnvelope(ctrl)
		core     = &coreService{dao: blkDAO}
		blk      = &block.Block{
			Header: block.Header{},
			Body: block.Body{
				Actions: []*action.SealedEnvelope{{Envelope: envelope}},
			},
			Footer:   block.Footer{},
			Receipts: nil,
		}
		receiptes = []*action.Receipt{{ActionHash: hash.ZeroHash256}}
	)

	t.Run("CheckParams", func(t *testing.T) {
		t.Run("ReverseStartGreaterThanSize", func(t *testing.T) {
			actions := core.reverseActionsInBlock(blk, 2, 1)
			require.Empty(actions)
		})

		t.Run("CountIsZero", func(t *testing.T) {
			actions := core.reverseActionsInBlock(blk, 1, 0)
			require.Empty(actions)
		})
	})

	t.Run("FailedToGetReceiptFromDAO", func(t *testing.T) {
		blkDAO.EXPECT().GetReceipts(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)
		actions := core.reverseActionsInBlock(blk, 0, 1)
		require.Empty(actions)
	})

	t.Run("ForeachActions", func(t *testing.T) {
		t.Run("FailedToHash", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			p = p.ApplyMethodReturn(&action.SealedEnvelope{}, "Hash", nil, errors.New(t.Name()))

			blkDAO.EXPECT().GetReceipts(gomock.Any()).Return(receiptes, nil).Times(1)

			actions := core.reverseActionsInBlock(blk, 0, 1)
			require.Empty(actions)
		})

		t.Run("FailedToGetReceiptFromMap", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			p = p.ApplyMethodReturn(&action.SealedEnvelope{}, "Hash", hash.BytesToHash256([]byte("test")), nil)

			blkDAO.EXPECT().GetReceipts(gomock.Any()).Return(receiptes, nil).Times(1)

			actions := core.reverseActionsInBlock(blk, 0, 1)
			require.Empty(actions)
		})

		t.Run("ReverseActionsInBlockSuccess", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			p = p.ApplyMethodReturn(&action.SealedEnvelope{}, "Hash", hash.ZeroHash256, nil)
			p = p.ApplyMethodReturn(&action.SealedEnvelope{}, "SenderAddress", identityset.Address(1))
			p = p.ApplyMethodReturn(&action.SealedEnvelope{}, "Proto", &iotextypes.Action{})
			p = p.ApplyMethodReturn(&block.Header{}, "BlockHeaderCoreProto", &iotextypes.BlockHeaderCore{Timestamp: timestamppb.Now()})

			blkDAO.EXPECT().GetReceipts(gomock.Any()).Return(receiptes, nil).Times(1)
			envelope.EXPECT().GasPrice().Return(big.NewInt(1)).Times(1)
			actions := core.reverseActionsInBlock(blk, 0, 1)
			require.Equal(1, len(actions))
		})
	})

	t.Run("StartIsNotZero", func(t *testing.T) {
		blkDAO.EXPECT().GetReceipts(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)
		blk.Actions = append(blk.Actions, &action.SealedEnvelope{})
		actions := core.reverseActionsInBlock(blk, 0, 1)
		require.Empty(actions)
	})
}

func TestActions(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	indexer := mock_blockindex.NewMockIndexer(ctrl)
	cs := &coreService{
		cfg:     DefaultConfig,
		indexer: indexer,
	}

	t.Run("FailedToCheckActionIndex", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		p = p.ApplyPrivateMethod(
			cs,
			"checkActionIndex",
			func() error {
				return errors.New(t.Name())
			},
		)
		_, err := cs.Actions(0, 0)
		require.EqualError(err, t.Name())
	})

	t.Run("CountIsZero", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		p = p.ApplyPrivateMethod(
			cs,
			"checkActionIndex",
			func() error {
				return nil
			},
		)

		_, err := cs.Actions(0, 0)
		require.ErrorContains(err, "count must be greater than zero")
	})

	t.Run("CountIsGreaterThanRange", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		p = p.ApplyPrivateMethod(
			cs,
			"checkActionIndex",
			func() error {
				return nil
			},
		)

		_, err := cs.Actions(0, 1001)
		require.ErrorContains(err, "range exceeds the limit")
	})

	t.Run("FailedToGetTotalActions", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		p = p.ApplyPrivateMethod(
			cs,
			"checkActionIndex",
			func() error {
				return nil
			},
		)

		indexer.EXPECT().GetTotalActions().Return(uint64(0), errors.New(t.Name())).Times(1)
		_, err := cs.Actions(0, 1)
		require.ErrorContains(err, t.Name())
	})

	t.Run("StartGreaterThanTotalActions", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		p = p.ApplyPrivateMethod(
			cs,
			"checkActionIndex",
			func() error {
				return nil
			},
		)

		// greater than totalActions
		indexer.EXPECT().GetTotalActions().Return(uint64(0), nil).Times(1)
		_, err := cs.Actions(1, 1)
		require.ErrorContains(err, "start exceeds the total actions in the block")

		// equal totalActions
		indexer.EXPECT().GetTotalActions().Return(uint64(2), nil).Times(1)
		_, err = cs.Actions(2, 1)
		require.ErrorContains(err, "start exceeds the total actions in the block")
	})

	t.Run("GetActionsFromIndexer", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		p = p.ApplyPrivateMethod(
			cs,
			"checkActionIndex",
			func() error {
				return nil
			},
		)

		indexer.EXPECT().GetTotalActions().Return(uint64(1), nil).Times(1)
		p = p.ApplyPrivateMethod(
			cs,
			"getActionsFromIndex",
			func(totalActions, start, count uint64) ([]*iotexapi.ActionInfo, error) {
				return nil, nil
			},
		)
		infos, err := cs.Actions(0, 1)
		require.NoError(err)
		require.Empty(infos)
	})
}

func TestCheckActionIndex(t *testing.T) {
	require := require.New(t)

	cs := &coreService{}
	t.Run("IndexerIsNil", func(t *testing.T) {
		err := cs.checkActionIndex()
		require.ErrorContains(err, "no action index")
	})
}

func TestReceiveBlock(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	listener := mock_apitypes.NewMockListener(ctrl)
	cs := &coreService{
		readCache:     &ReadCache{},
		chainListener: listener,
	}

	t.Run("FailedToReceiveBlock", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		p = p.ApplyMethodReturn(cs.readCache, "Clear")
		listener.EXPECT().ReceiveBlock(gomock.Any()).Return(errors.New(t.Name())).Times(1)
		err := cs.ReceiveBlock(&block.Block{})
		require.ErrorContains(err, t.Name())
	})

	t.Run("ReceiveBlockSuccess", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		p = p.ApplyMethodReturn(cs.readCache, "Clear")
		listener.EXPECT().ReceiveBlock(gomock.Any()).Return(nil).Times(1)
		err := cs.ReceiveBlock(&block.Block{})
		require.NoError(err)
	})
}

func TestSimulateExecution(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		bc  = mock_blockchain.NewMockBlockchain(ctrl)
		dao = mock_blockdao.NewMockBlockDAO(ctrl)
		sf  = mock_factory.NewMockFactory(ctrl)
		cs  = &coreService{
			bc:  bc,
			dao: dao,
			sf:  sf,
		}
		ctx = context.Background()
	)

	t.Run("FailedToAccountState", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		p = p.ApplyFuncReturn(accountutil.AccountState, nil, errors.New(t.Name()))
		bc.EXPECT().Genesis().Return(genesis.Genesis{}).Times(1)
		bc.EXPECT().TipHeight().Return(uint64(1)).Times(1)
		bc.EXPECT().Context(gomock.Any()).Return(ctx, nil).Times(1)
		sf.EXPECT().WorkingSet(gomock.Any()).Return(nil, nil).Times(1)
		elp := (&action.EnvelopeBuilder{}).SetAction(&action.Execution{}).Build()
		_, _, err := cs.SimulateExecution(ctx, &address.AddrV1{}, elp)
		require.ErrorContains(err, t.Name())
	})

	t.Run("FailedToContext", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		p = p.ApplyFuncReturn(accountutil.AccountState, &state.Account{}, nil)
		bc.EXPECT().Genesis().Return(genesis.Genesis{}).Times(1)
		bc.EXPECT().TipHeight().Return(uint64(1)).Times(1)
		bc.EXPECT().Context(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)
		elp := (&action.EnvelopeBuilder{}).SetAction(&action.Execution{}).Build()
		_, _, err := cs.SimulateExecution(ctx, &address.AddrV1{}, elp)
		require.ErrorContains(err, t.Name())
	})

	t.Run("Success", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		p = p.ApplyFuncReturn(accountutil.AccountState, &state.Account{}, nil)
		p = p.ApplyMethodReturn(&genesis.Blockchain{}, "BlockGasLimitByHeight", uint64(0))
		p = p.ApplyPrivateMethod(
			cs,
			"simulateExecution",
			func(ctx context.Context, addr address.Address, exec *action.Execution, getBlockHash evm.GetBlockHash, getBlockTime evm.GetBlockTime) ([]byte, *action.Receipt, error) {
				return []byte("success"), nil, nil
			},
		)

		bc.EXPECT().Genesis().Return(genesis.Genesis{}).Times(1)
		bc.EXPECT().TipHeight().Return(uint64(0)).Times(1)
		elp := (&action.EnvelopeBuilder{}).SetAction(&action.Execution{}).Build()
		bytes, _, err := cs.SimulateExecution(ctx, &address.AddrV1{}, elp)
		require.NoError(err)
		require.Equal([]byte("success"), bytes)
	})
}

func TestSyncingProgress(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bs := mock_blocksync.NewMockBlockSync(ctrl)
	cs := &coreService{bs: bs}
	bs.EXPECT().SyncStatus().Return(uint64(0), uint64(0), uint64(0), "").Times(1)
	startingHeight, currentHeight, targetHeight := cs.SyncingProgress()
	require.Equal(uint64(0), startingHeight)
	require.Equal(uint64(0), currentHeight)
	require.Equal(uint64(0), targetHeight)
}

func TestTrack(t *testing.T) {
	cs := &coreService{}
	t.Run("ApiStatsIsNil", func(t *testing.T) {
		cs.Track(nil, time.Now(), "", 0, true)
	})

	cs.apiStats = &nodestats.APILocalStats{}
	t.Run("ApiStatsIsNotNil", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		p = p.ApplyFuncReturn(time.Since, time.Duration(0))
		p = p.ApplyMethodReturn(cs.apiStats, "ReportCall")
		cs.Track(nil, time.Now(), "", 0, true)
	})
}

func TestTraceTx(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		bc = mock_blockchain.NewMockBlockchain(ctrl)
		cs = &coreService{
			bc: bc,
		}
		ctx = context.Background()
	)

	t.Run("ConfigIsNil", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		p = p.ApplyFuncReturn(logger.NewStructLogger, nil)
		p = p.ApplyFuncReturn(protocol.WithVMConfigCtx, ctx)
		p = p.ApplyFuncReturn(protocol.WithBlockCtx, ctx)
		p = p.ApplyFuncReturn(genesis.WithGenesisContext, ctx)
		p = p.ApplyFuncReturn(protocol.WithBlockchainCtx, ctx)
		p = p.ApplyFuncReturn(protocol.WithFeatureCtx, ctx)
		retval, receipt, tracer, err := cs.traceTx(ctx, nil, nil, func(ctx context.Context) ([]byte, *action.Receipt, error) {
			return nil, nil, nil
		})
		require.NoError(err)
		require.Empty(retval)
		require.Empty(receipt)
		require.Empty(tracer)
	})

	t.Run("TracerIsNotNil", func(t *testing.T) {

		t.Run("FailedToParseDuration", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			p = p.ApplyFuncReturn(time.ParseDuration, nil, errors.New(t.Name()))

			testStr := "TestTracer"
			_, _, _, err := cs.traceTx(ctx, nil, &tracers.TraceConfig{Tracer: &testStr, Timeout: &testStr}, func(ctx context.Context) ([]byte, *action.Receipt, error) {
				return nil, nil, nil
			})
			require.ErrorContains(err, t.Name())
		})

		t.Run("FailedToNewTracer", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			p = p.ApplyMethodReturn(&tracers.DefaultDirectory, "New", nil, errors.New(t.Name()))
			testStr := "TestTracer"
			_, _, _, err := cs.traceTx(ctx, nil, &tracers.TraceConfig{Tracer: &testStr}, func(ctx context.Context) ([]byte, *action.Receipt, error) {
				return nil, nil, nil
			})
			require.ErrorContains(err, t.Name())
		})
	})

	t.Run("TracerIsNil", func(t *testing.T) {
		p := NewPatches()
		defer p.Reset()

		p = p.ApplyFuncReturn(logger.NewStructLogger, nil)
		p = p.ApplyFuncReturn(protocol.WithVMConfigCtx, ctx)
		p = p.ApplyFuncReturn(protocol.WithBlockCtx, ctx)
		p = p.ApplyFuncReturn(genesis.WithGenesisContext, ctx)
		p = p.ApplyFuncReturn(protocol.WithBlockchainCtx, ctx)
		p = p.ApplyFuncReturn(protocol.WithFeatureCtx, ctx)
		retval, receipt, tracer, err := cs.traceTx(ctx, nil, &tracers.TraceConfig{}, func(ctx context.Context) ([]byte, *action.Receipt, error) {
			return nil, nil, nil
		})
		require.NoError(err)
		require.Empty(retval)
		require.Empty(receipt)
		require.Empty(tracer)
	})
}
