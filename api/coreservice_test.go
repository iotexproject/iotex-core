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
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

func TestLogsInRange(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer()
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

func BenchmarkLogsInRange(b *testing.B) {
	svr, _, _, _, cleanCallback := setupTestServer()
	defer cleanCallback()

	testData := &filterObject{FromBlock: "0x1"}
	filter, _ := getTopicsAddress(testData.Address, testData.Topics)
	from, _ := strconv.ParseInt(testData.FromBlock, 10, 64)
	to, _ := strconv.ParseInt(testData.ToBlock, 10, 64)

	b.Run("five workers to extract logs", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			svr.web3Server.coreService.LogsInRange(logfilter.NewLogFilter(&filter, nil, nil), uint64(from), uint64(to), uint64(0))
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
