// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	_evmNetworkID uint32 = 1
)

func TestWeb3ServerIntegrity(t *testing.T) {
	svr, bc, dao, actPool, cleanIndexFile := setupTestServer()
	web3svr := svr.httpSvr
	defer cleanIndexFile()
	ctx := context.Background()
	web3svr.Start(ctx)
	defer web3svr.Stop(ctx)
	handler := newHTTPHandler(NewWeb3Handler(svr.core, ""))

	// send request
	t.Run("eth_gasPrice", func(t *testing.T) {
		gasPrice(t, handler)
	})

	t.Run("eth_chainId", func(t *testing.T) {
		chainID(t, handler)
	})

	t.Run("eth_blockNumber", func(t *testing.T) {
		blockNumber(t, handler)
	})

	t.Run("eth_getBlockByNumber", func(t *testing.T) {
		getBlockByNumber(t, handler)
	})

	t.Run("eth_getBalance", func(t *testing.T) {
		getBalance(t, handler)
	})

	t.Run("eth_getTransactionCount", func(t *testing.T) {
		getTransactionCount(t, handler)
	})

	t.Run("eth_call", func(t *testing.T) {
		ethCall(t, handler)
	})

	t.Run("web3_clientVersion", func(t *testing.T) {
		getNodeInfo(t, handler)
	})

	t.Run("eth_getBlockTransactionCountByHash", func(t *testing.T) {
		getBlockTransactionCountByHash(t, handler, bc)
	})

	t.Run("eth_getBlockByHash", func(t *testing.T) {
		getBlockByHash(t, handler, bc)
	})

	t.Run("eth_getTransactionByHash", func(t *testing.T) {
		getTransactionByHash(t, handler)
	})

	t.Run("eth_getLogs", func(t *testing.T) {
		getLogs(t, handler)
	})

	t.Run("eth_getTransactionReceipt", func(t *testing.T) {
		getTransactionReceipt(t, handler)
	})

	t.Run("eth_getBlockTransactionCountByNumber", func(t *testing.T) {
		getBlockTransactionCountByNumber(t, handler)
	})

	t.Run("eth_getTransactionByBlockHashAndIndex", func(t *testing.T) {
		getTransactionByBlockHashAndIndex(t, handler, bc)
	})

	t.Run("eth_getTransactionByBlockNumberAndIndex", func(t *testing.T) {
		getTransactionByBlockNumberAndIndex(t, handler)
	})

	t.Run("eth_newFilter", func(t *testing.T) {
		newfilter(t, handler)
	})

	t.Run("eth_newBlockFilter", func(t *testing.T) {
		newBlockFilter(t, handler)
	})

	t.Run("eth_getFilterChanges", func(t *testing.T) {
		getFilterChanges(t, handler)
	})

	t.Run("eth_getFilterLogs", func(t *testing.T) {
		getFilterLogs(t, handler)
	})

	t.Run("net_version", func(t *testing.T) {
		getNetworkID(t, handler)
	})

	t.Run("eth_accounts", func(t *testing.T) {
		ethAccounts(t, handler)
	})

	t.Run("web3Staking", func(t *testing.T) {
		web3Staking(t, handler)
	})

	t.Run("eth_sendRawTransaction", func(t *testing.T) {
		sendRawTransaction(t, handler)
	})

	t.Run("eth_estimateGas", func(t *testing.T) {
		estimateGas(t, handler, bc, dao, actPool)
	})

	t.Run("eth_getCode", func(t *testing.T) {
		getCode(t, handler, bc, dao, actPool)
	})

	t.Run("eth_getStorageAt", func(t *testing.T) {
		getStorageAt(t, handler, bc, dao, actPool)
	})
}

func setupTestServer() (*ServerV2, blockchain.Blockchain, blockdao.BlockDAO, actpool.ActPool, func()) {
	cfg := newConfig()
	cfg.Chain.EVMNetworkID = _evmNetworkID
	svr, bc, dao, _, _, actPool, bfIndexFile, _ := createServerV2ForHttp(cfg, false)
	return svr, bc, dao, actPool, func() {
		testutil.CleanupPath(bfIndexFile)
	}
}

func serveTestHTTP(require *require.Assertions, handler *hTTPHandler, method string, param string) interface{} {
	req, _ := http.NewRequest(http.MethodPost, "http://url.com",
		strings.NewReader(fmt.Sprintf(`[{"jsonrpc":"2.0","method":"%s","params":%s,"id":1}]`, method, param)))
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	var vals []struct {
		Jsonrpc string      `json:"jsonrpc"`
		ID      int         `json:"id"`
		Result  interface{} `json:"result"`
	}
	err := json.NewDecoder(resp.Body).Decode(&vals)
	require.NoError(err)
	require.NotEmpty(vals)
	return vals[0].Result
}

func gasPrice(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "eth_gasPrice", "[]")
	actual, ok := result.(string)
	require.True(ok)
	require.Equal(uint64ToHex(1000000000000), actual)
}

func chainID(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "eth_chainId", "[]")
	actual, ok := result.(string)
	require.True(ok)
	require.Equal(uint64ToHex(1), actual)
	require.Equal(uint64ToHex(uint64(_evmNetworkID)), actual)
}

func blockNumber(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "eth_blockNumber", "[]")
	actual, ok := result.(string)
	require.True(ok)
	require.Equal(uint64ToHex(4), actual)
}

func getBlockByNumber(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	for _, test := range []struct {
		params   string
		expected int
	}{
		{`["1", true]`, 1},
		{`["1", false]`, 2},
		{`["10", false]`, 0},
	} {
		result := serveTestHTTP(require, handler, "eth_getBlockByNumber", test.params)
		if test.expected == 0 {
			require.Nil(result)
		} else {
			actual, ok := result.(map[string]interface{})["transactions"]
			require.True(ok)
			require.Equal(test.expected, len(actual.([]interface{})))
		}
	}
}

func getBalance(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "eth_getBalance", `["0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", 1]`)
	ans, ok := new(big.Int).SetString("9999999999999999999999999991", 10)
	require.True(ok)
	actual, ok := result.(string)
	require.True(ok)
	ans, ok = new(big.Int).SetString("9999999999999999999999999991", 10)
	require.True(ok)
	require.Equal("0x"+fmt.Sprintf("%x", ans), actual)
}

func getTransactionCount(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	for _, test := range []struct {
		params   string
		expected int
	}{
		{`["0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", "0x1"]`, 2},
		{`["0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", "pending"]`, 2},
	} {
		result := serveTestHTTP(require, handler, "eth_getTransactionCount", test.params)
		actual, ok := result.(string)
		require.True(ok)
		require.Equal(uint64ToHex(uint64(test.expected)), actual)
	}
}

func ethCall(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	for _, test := range []struct {
		params   string
		expected int
	}{
		{
			`[{
				"from":     "",
				"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
				"gas":      "0x4e20",
				"gasPrice": "0xe8d4a51000",
				"value":    "0x1",
				"data":     "0x1"
			  },
			1]`,
			1,
		},
		{
			`[{
				"from":     "",
				"to":       "0xb1f8e55c7f64d203c1400b9d8555d050f94adf39",
				"gas":      "0x4e20",
				"gasPrice": "0xe8d4a51000",
				"value":    "0x1",
				"data":     "0x1"
			   },
			1]`,
			0,
		},
	} {
		result := serveTestHTTP(require, handler, "eth_call", test.params)
		if test.expected == 0 {
			require.Nil(result)
			return
		}
		actual, ok := result.(string)
		require.True(ok)
		require.Equal("0x", actual)
	}
}

func getNodeInfo(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "web3_clientVersion", "[]")
	actual, ok := result.(string)
	require.True(ok)
	require.Equal("NoBuildInfo/NoBuildInfo", actual)
}

func getBlockTransactionCountByHash(t *testing.T, handler *hTTPHandler, bc blockchain.Blockchain) {
}

func getBlockByHash(t *testing.T, handler *hTTPHandler, bc blockchain.Blockchain) {
}

func getTransactionByHash(t *testing.T, handler *hTTPHandler) {
}

func getLogs(t *testing.T, handler *hTTPHandler) {
}

func getTransactionReceipt(t *testing.T, handler *hTTPHandler) {
}

func getBlockTransactionCountByNumber(t *testing.T, handler *hTTPHandler) {
}

func getTransactionByBlockHashAndIndex(t *testing.T, handler *hTTPHandler, bc blockchain.Blockchain) {
}

func getTransactionByBlockNumberAndIndex(t *testing.T, handler *hTTPHandler) {
}

func newfilter(t *testing.T, handler *hTTPHandler) {
}

func newBlockFilter(t *testing.T, handler *hTTPHandler) {
}

func getFilterChanges(t *testing.T, handler *hTTPHandler) {
}

func getFilterLogs(t *testing.T, handler *hTTPHandler) {
}

func getNetworkID(t *testing.T, handler *hTTPHandler) {
}

func ethAccounts(t *testing.T, handler *hTTPHandler) {
}

func web3Staking(t *testing.T, handler *hTTPHandler) {
}

func sendRawTransaction(t *testing.T, handler *hTTPHandler) {
}

func estimateGas(t *testing.T, handler *hTTPHandler, bc blockchain.Blockchain, dao blockdao.BlockDAO, actPool actpool.ActPool) {
}

func getCode(t *testing.T, handler *hTTPHandler, bc blockchain.Blockchain, dao blockdao.BlockDAO, actPool actpool.ActPool) {
}

func getStorageAt(t *testing.T, handler *hTTPHandler, bc blockchain.Blockchain, dao blockdao.BlockDAO, actPool actpool.ActPool) {
	require := require.New(t)
	// deploy a contract
	contractCode := "608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c14610057575b600080fd5b6100556004803603810190610050919061009d565b610075565b005b61005f61007f565b60405161006c91906100d9565b60405180910390f35b8060008190555050565b60008054905090565b60008135905061009781610103565b92915050565b6000602082840312156100b3576100b26100fe565b5b60006100c184828501610088565b91505092915050565b6100d3816100f4565b82525050565b60006020820190506100ee60008301846100ca565b92915050565b6000819050919050565b600080fd5b61010c816100f4565b811461011757600080fd5b5056fea2646970667358221220c86a8c4dd175f55f5732b75b721d714ceb38a835b87c6cf37cf28c790813e19064736f6c63430008070033"
	contract, _ := deployContractV2(bc, dao, actPool, identityset.PrivateKey(13), 1, bc.TipHeight(), contractCode)
	contractAddr, _ := ioAddrToEthAddr(contract)

	for _, test := range []struct {
		params   string
		expected int
	}{
		{fmt.Sprintf(`["%s", "0x0"]`, contractAddr), 1},
		{`[1]`, 0},
		{`["TEST", "TEST"]`, 0},
	} {
		result := serveTestHTTP(require, handler, "eth_getStorageAt", test.params)
		if test.expected == 0 {
			require.Nil(result)
			return
		}
		actual, ok := result.(string)
		require.True(ok)
		// the value of any contract at pos0 is be "0x0000000000000000000000000000000000000000000000000000000000000000"
		require.Equal("0x0000000000000000000000000000000000000000000000000000000000000000", actual)
	}
}
