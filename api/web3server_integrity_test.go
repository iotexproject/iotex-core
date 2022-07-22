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

	t.Run("TestGasPriceIntegrity", func(t *testing.T) {
		gasPriceIntegrity(t, handler)
	})

	t.Run("TestGetChainIDIntegrity", func(t *testing.T) {
		chainIDIntegrity(t, handler)
	})

	t.Run("TestGetBlockNumberIntegrity", func(t *testing.T) {
		blockNumberIntegrity(t, handler)
	})

	t.Run("TestGetBlockByNumberIntegrity", func(t *testing.T) {
		blockByNumberIntegrity(t, handler)
	})

	t.Run("TestGetBalanceIntegrity", func(t *testing.T) {
		balanceIntegrity(t, handler)
	})

	t.Run("TestGetTransactionCountIntegrity", func(t *testing.T) {
		transactionCountIntegrity(t, handler)
	})

	t.Run("TestCallIntegrity", func(t *testing.T) {
		callIntegrity(t, handler)
	})

	t.Run("TestGetNodeInfoIntegrity", func(t *testing.T) {
		nodeInfoIntegrity(t, handler)
	})

	t.Run("TestGetStorageAtIntegrity", func(t *testing.T) {
		storageAtIntegrity(t, handler, bc, dao, actPool)
	})
}

func setupTestServer() (*ServerV2, blockchain.Blockchain, blockdao.BlockDAO, actpool.ActPool, func()) {
	cfg := newConfig()
	cfg.Chain.EVMNetworkID = _evmNetworkID
	svr, bc, dao, _, _, actPool, bfIndexFile, _ := createServerV2(cfg, false)
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

func gasPriceIntegrity(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "eth_gasPrice", "[]")
	actual, ok := result.(string)
	require.True(ok)
	require.Equal(uint64ToHex(1000000000000), actual)
}

func chainIDIntegrity(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "eth_chainId", "[]")
	actual, ok := result.(string)
	require.True(ok)
	require.Equal(uint64ToHex(1), actual)
	require.Equal(uint64ToHex(uint64(_evmNetworkID)), actual)
}

func blockNumberIntegrity(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "eth_blockNumber", "[]")
	actual, ok := result.(string)
	require.True(ok)
	require.Equal(uint64ToHex(4), actual)
}

func blockByNumberIntegrity(t *testing.T, handler *hTTPHandler) {
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

func balanceIntegrity(t *testing.T, handler *hTTPHandler) {
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

func transactionCountIntegrity(t *testing.T, handler *hTTPHandler) {
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

func callIntegrity(t *testing.T, handler *hTTPHandler) {
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

func nodeInfoIntegrity(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "web3_clientVersion", "[]")
	actual, ok := result.(string)
	require.True(ok)
	require.Equal("NoBuildInfo/NoBuildInfo", actual)
}

func storageAtIntegrity(t *testing.T, handler *hTTPHandler, bc blockchain.Blockchain, dao blockdao.BlockDAO, actPool actpool.ActPool) {
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
