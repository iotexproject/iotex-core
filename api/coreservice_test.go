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

	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/ethereum/go-ethereum/eth/tracers/logger"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockindex"
	"github.com/iotexproject/iotex-core/testutil"
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
	svr, err := newCoreService(cfg.api, bc, nil, sf, dao, indexer, bfIndexer, ap, registry, func(u uint64) (time.Time, error) { return time.Time{}, nil }, opts...)
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

func TestEstimateExecutionGasConsumption(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svr, _, _, _, cleanCallback := setupTestCoreService()
	defer cleanCallback()

	callAddr := identityset.Address(29)
	sc, err := action.NewExecution("", 0, big.NewInt(0), 0, big.NewInt(0), []byte{})
	require.NoError(err)

	//gasprice is zero
	sc.SetGasPrice(big.NewInt(0))
	estimatedGas, err := svr.EstimateExecutionGasConsumption(context.Background(), sc, callAddr)
	require.NoError(err)
	require.Equal(uint64(10000), estimatedGas)

	//gasprice no zero, should return error before fixed
	sc.SetGasPrice(big.NewInt(100))
	estimatedGas, err = svr.EstimateExecutionGasConsumption(context.Background(), sc, callAddr)
	require.NoError(err)
	require.Equal(uint64(10000), estimatedGas)

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
