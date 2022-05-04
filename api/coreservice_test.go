// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/actpool"
	logfilter "github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockindex"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

func TestLogsInRange(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestCoreSerivce()
	defer cleanCallback()

	t.Run("blocks with four logs", func(t *testing.T) {
		testData := &filterObject{FromBlock: "1", ToBlock: "4"}
		filter, err := getTopicsAddress(testData.Address, testData.Topics)
		require.NoError(err)
		from, err := strconv.ParseUint(testData.FromBlock, 10, 64)
		require.NoError(err)
		to, err := strconv.ParseUint(testData.ToBlock, 10, 64)
		require.NoError(err)

		logs, hashes, err := svr.LogsInRange(logfilter.NewLogFilter(&filter), from, to, uint64(0))
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

		logs, hashes, err := svr.LogsInRange(logfilter.NewLogFilter(&filter), from, to, uint64(0))
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

		logs, hashes, err := svr.LogsInRange(logfilter.NewLogFilter(&filter), from, to, uint64(5001))
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

		_, _, err = svr.LogsInRange(logfilter.NewLogFilter(&filter), from, to, uint64(0))
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

		_, _, err = svr.LogsInRange(logfilter.NewLogFilter(&filter), from, to, uint64(0))
		expectedErr := errors.New("start block > tip height")
		require.Error(err)
		require.Equal(expectedErr.Error(), err.Error())
	})
}

func BenchmarkLogsInRange(b *testing.B) {
	svr, _, _, _, cleanCallback := setupTestCoreSerivce()
	defer cleanCallback()

	ctrl := gomock.NewController(b)
	defer ctrl.Finish()
	blk := mock_blockindex.NewMockBloomFilterIndexer(ctrl)

	testData := &filterObject{FromBlock: "0x1"}
	filter, _ := getTopicsAddress(testData.Address, testData.Topics)
	from, _ := strconv.ParseInt(testData.FromBlock, 10, 64)
	to, _ := strconv.ParseInt(testData.ToBlock, 10, 64)

	b.Run("five workers to extract logs", func(b *testing.B) {
		blk.EXPECT().FilterBlocksInRange(logfilter.NewLogFilter(&filter), uint64(from), uint64(to)).Return([]uint64{1, 2, 3, 4}, nil).AnyTimes()
		for i := 0; i < b.N; i++ {
			svr.LogsInRange(logfilter.NewLogFilter(&filter), uint64(from), uint64(to), uint64(0))
		}
	})
}

func getTopicsAddress(addr []string, topics [][]string) (iotexapi.LogsFilter, error) {
	var filter iotexapi.LogsFilter
	for _, ethAddr := range addr {
		ioAddr, err := ethAddrToIoAddr(ethAddr)
		if err != nil {
			return iotexapi.LogsFilter{}, err
		}
		filter.Address = append(filter.Address, ioAddr.String())
	}
	for _, tp := range topics {
		var topic [][]byte
		for _, str := range tp {
			b, err := hexToBytes(str)
			if err != nil {
				return iotexapi.LogsFilter{}, err
			}
			topic = append(topic, b)
		}
		filter.Topics = append(filter.Topics, &iotexapi.Topics{Topic: topic})
	}

	return filter, nil
}

func setupTestCoreSerivce() (CoreService, blockchain.Blockchain, blockdao.BlockDAO, actpool.ActPool, func()) {
	cfg := newConfig()
	config.SetEVMNetworkID(_evmNetworkID)

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
	if _, ok := cfg.Plugins[config.GatewayPlugin]; ok {
		opts = append(opts, WithActionIndex())
	}
	svr, err := newCoreService(cfg.API, bc, nil, sf, dao, indexer, bfIndexer, ap, registry, opts...)
	if err != nil {
		panic(err)
	}

	return svr, bc, dao, ap, func() {
		testutil.CleanupPath(bfIndexFile)
	}
}
