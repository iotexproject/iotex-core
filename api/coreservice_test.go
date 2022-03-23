// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"fmt"
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

	testData := []struct {
		filter      *filterObject
		description string
		logLen      int
	}{
		{
			&filterObject{FromBlock: "1", ToBlock: "4"},
			"blocks with four logs",
			4,
		},
		{
			&filterObject{FromBlock: "0x1"},
			"default value for invalid string FromBlock & ToBlock",
			4,
		},
		{
			// empty log
			&filterObject{FromBlock: "5", ToBlock: "1"},
			"invalid start and end height",
			0,
		},
		{
			// empty log
			&filterObject{FromBlock: "5", ToBlock: "5"},
			"start block > tip height",
			0,
		},
		{
			// empty log
			&filterObject{Address: []string{"0x8ce313ab12bf7aed8136ab36c623ff98c8eaad34"}, FromBlock: "1", ToBlock: "4"},
			"empty log",
			0,
		},
		{
			&filterObject{FromBlock: "1", ToBlock: "4"},
			"over 5000 pagenation size",
			4,
		},
	}
	for i, d := range testData {
		t.Run(fmt.Sprintf("%v", d.description), func(t *testing.T) {
			var filter iotexapi.LogsFilter
			for _, ethAddr := range d.filter.Address {
				ioAddr, err := ethAddrToIoAddr(ethAddr)
				require.NoError(err)
				filter.Address = append(filter.Address, ioAddr.String())
			}
			for _, tp := range d.filter.Topics {
				var topic [][]byte
				for _, str := range tp {
					b, err := hexToBytes(str)
					require.NoError(err)
					topic = append(topic, b)
				}
				filter.Topics = append(filter.Topics, &iotexapi.Topics{Topic: topic})
			}

			// LogsInRange will assign default value if from, to returns error
			from, err := strconv.ParseInt(d.filter.FromBlock, 10, 64)
			if i == 1 {
				require.Error(err)
			} else {
				fmt.Println(i)
				require.NoError(err)
			}
			to, err := strconv.ParseInt(d.filter.ToBlock, 10, 64)
			if i == 1 {
				require.Error(err)
			} else {
				require.NoError(err)
			}
			pagenationSize := 0
			if i == 5 {
				pagenationSize = 5001
			}
			logs, err := svr.web3Server.coreService.LogsInRange(logfilter.NewLogFilter(&filter, nil, nil), uint64(from), uint64(to), uint64(pagenationSize))
			if i == 2 {
				expectedErr := errors.New("invalid start and end height")
				expectedValue := []*iotextypes.Log(nil)
				require.Error(err)
				require.Equal(expectedErr.Error(), err.Error())
				require.Equal(expectedValue, logs)
			} else if i == 3 {
				exprectedErr := errors.New("start block > tip height")
				expectedValue := []*iotextypes.Log(nil)
				require.Error(err)
				require.Equal(exprectedErr.Error(), err.Error())
				require.Equal(expectedValue, logs)
			} else {
				require.NoError(err)
				require.Equal(d.logLen, len(logs))
			}
		})
	}
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
