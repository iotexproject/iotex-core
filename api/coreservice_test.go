// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"strconv"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	logfilter "github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

func TestLogsInRange(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	t.Run("blocks with four logs", func(t *testing.T) {
		testData := &filterObject{FromBlock: "1", ToBlock: "4"}
		filter, err := getTopicsAddress(testData.Address, testData.Topics)
		require.NoError(err)
		from, err := strconv.ParseUint(testData.FromBlock, 10, 64)
		require.NoError(err)
		to, err := strconv.ParseUint(testData.ToBlock, 10, 64)
		require.NoError(err)

		logs, err := svr.web3Server.coreService.LogsInRange(logfilter.NewLogFilter(&filter, nil, nil), from, to, uint64(0))
		require.NoError(err)
		require.Equal(4, len(logs))
	})
	t.Run("empty log", func(t *testing.T) {
		testData := &filterObject{FromBlock: "1", ToBlock: "4", Address: []string{"0x8ce313ab12bf7aed8136ab36c623ff98c8eaad34"}}
		filter, err := getTopicsAddress(testData.Address, testData.Topics)
		require.NoError(err)
		from, err := strconv.ParseUint(testData.FromBlock, 10, 64)
		require.NoError(err)
		to, err := strconv.ParseUint(testData.ToBlock, 10, 64)
		require.NoError(err)

		logs, err := svr.web3Server.coreService.LogsInRange(logfilter.NewLogFilter(&filter, nil, nil), from, to, uint64(0))
		require.NoError(err)
		require.Equal(0, len(logs))
	})
	t.Run("over 5000 pagenation size", func(t *testing.T) {
		testData := &filterObject{FromBlock: "1", ToBlock: "4"}
		filter, err := getTopicsAddress(testData.Address, testData.Topics)
		require.NoError(err)
		from, err := strconv.ParseUint(testData.FromBlock, 10, 64)
		require.NoError(err)
		to, err := strconv.ParseUint(testData.ToBlock, 10, 64)
		require.NoError(err)

		logs, err := svr.web3Server.coreService.LogsInRange(logfilter.NewLogFilter(&filter, nil, nil), from, to, uint64(5001))
		require.NoError(err)
		require.Equal(4, len(logs))
	})
	t.Run("invalid start and end height", func(t *testing.T) {
		testData := &filterObject{FromBlock: "2", ToBlock: "1"}
		filter, err := getTopicsAddress(testData.Address, testData.Topics)
		require.NoError(err)
		from, err := strconv.ParseUint(testData.FromBlock, 10, 64)
		require.NoError(err)
		to, err := strconv.ParseUint(testData.ToBlock, 10, 64)
		require.NoError(err)

		logs, err := svr.web3Server.coreService.LogsInRange(logfilter.NewLogFilter(&filter, nil, nil), from, to, uint64(0))
		expectedErr := errors.New("invalid start and end height")
		expectedValue := []*iotextypes.Log(nil)
		require.Error(err)
		require.Equal(expectedErr.Error(), err.Error())
		require.Equal(expectedValue, logs)
	})
	t.Run("start block > tip height", func(t *testing.T) {
		testData := &filterObject{FromBlock: "5", ToBlock: "5"}
		filter, err := getTopicsAddress(testData.Address, testData.Topics)
		require.NoError(err)
		from, err := strconv.ParseUint(testData.FromBlock, 10, 64)
		require.NoError(err)
		to, err := strconv.ParseUint(testData.ToBlock, 10, 64)
		require.NoError(err)

		logs, err := svr.web3Server.coreService.LogsInRange(logfilter.NewLogFilter(&filter, nil, nil), from, to, uint64(0))
		expectedErr := errors.New("start block > tip height")
		expectedValue := []*iotextypes.Log(nil)
		require.Error(err)
		require.Equal(expectedErr.Error(), err.Error())
		require.Equal(expectedValue, logs)
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

func BenchmarkLogsInRange(b *testing.B) {
	cfg := config.Default

	testTriePath, _ := testutil.PathOfTempFile("trie")
	testDBPath, _ := testutil.PathOfTempFile("db")
	testIndexPath, _ := testutil.PathOfTempFile("index")
	testSystemLogPath, _ := testutil.PathOfTempFile("systemlog")
	defer func() {
		testutil.CleanupPath(testTriePath)
		testutil.CleanupPath(testDBPath)
		testutil.CleanupPath(testIndexPath)
		testutil.CleanupPath(testSystemLogPath)
	}()

	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.System.SystemLogDBPath = testSystemLogPath
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.EnableGravityChainVoting = true
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.API.RangeQueryLimit = 100

	config.SetEVMNetworkID(_evmNetworkID)
	svr, _, _, _, _, _, bfIndexFile, _ := createServerV2(cfg, false)

	defer func() {
		testutil.CleanupPath(bfIndexFile)
	}()

	filter := &filterObject{FromBlock: "0x1"}
	from, _ := strconv.ParseInt(filter.FromBlock, 10, 64)
	to, _ := strconv.ParseInt(filter.ToBlock, 10, 64)
	var filters iotexapi.LogsFilter
	for _, ethAddr := range filter.Address {
		ioAddr, _ := ethAddrToIoAddr(ethAddr)
		filters.Address = append(filters.Address, ioAddr.String())
	}
	for _, tp := range filter.Topics {
		var topic [][]byte
		for _, str := range tp {
			b, _ := hexToBytes(str)
			topic = append(topic, b)
		}
		filters.Topics = append(filters.Topics, &iotexapi.Topics{Topic: topic})
	}

	b.Run("five workers to extract logs", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			svr.web3Server.coreService.LogsInRange(logfilter.NewLogFilter(&filters, nil, nil), uint64(from), uint64(to), uint64(0))
		}
	})
}
