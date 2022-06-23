// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
	"github.com/iotexproject/go-pkgs/util"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
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
	// web3svr, bc, dao, actPool, cleanCallback := setupTestServer()
	svr, _, _, _, cleanCallback := setupTestWeb3Server()
	web3svr := svr.web3Server
	defer cleanCallback()
	ctx := context.Background()
	web3svr.Start(ctx)
	defer web3svr.Stop(ctx)

	t.Run("TestGasPriceIntegrity", func(t *testing.T) {
		gasPriceIntegrity(t, web3svr)
	})

	t.Run("TestGetChainIDIntegrity", func(t *testing.T) {
		chainIDIntegrity(t, web3svr)
	})

	t.Run("TestGetBlockNumberIntegrity", func(t *testing.T) {
		blockNumberIntegrity(t, web3svr)
	})

	t.Run("TestGetBalanceIntegrity", func(t *testing.T) {
		balanceIntegrity(t, web3svr)
	})

	t.Run("TestGetBlockByNumberIntegrity", func(t *testing.T) {
		blockByNumberIntegrity(t, web3svr)
	})

	t.Run("TestGetTransactionCountIntegrity", func(t *testing.T) {
		transactionCountIntegrity(t, web3svr)
	})

	t.Run("TestCallIntegrity", func(t *testing.T) {
		callIntegrity(t, web3svr)
	})

	t.Run("TestGetNodeInfoIntegrity", func(t *testing.T) {
		nodeInfoIntegrity(t, web3svr)
	})

	t.Run("TestGetBlockTransactionCountByHashIntegrity", func(t *testing.T) {
		blockTransactionCountByHashIntegrity(t, web3svr, bc)
	})

	t.Run("TestGetBlockByHashIntegrity", func(t *testing.T) {
		blockByHashIntegrity(t, web3svr, bc)
	})

	t.Run("TestGetTransactionByHashIntegrity", func(t *testing.T) {
		transactionByHashIntegrity(t, web3svr)
	})

	t.Run("TestGetLogsIntegrity", func(t *testing.T) {
		logsIntegrity(t, web3svr)
	})

	t.Run("TestGetTransactionReceiptIntegrity", func(t *testing.T) {
		transactionReceiptIntegrity(t, web3svr)
	})

	t.Run("TestGetBlockTransactionCountByNumberIntegrity", func(t *testing.T) {
		blockTransactionCountByNumberIntegrity(t, web3svr)
	})

	t.Run("TestGetTransactionByBlockHashAndIndexIntegrity", func(t *testing.T) {
		transactionByBlockHashAndIndexIntegrity(t, web3svr, bc)
	})

	t.Run("TestGetTransactionByBlockNumberAndIndexIntegrity", func(t *testing.T) {
		transactionByBlockNumberAndIndexIntegrity(t, web3svr)
	})

	t.Run("TestNewfilterIntegrity", func(t *testing.T) {
		newfilterIntegrity(t, web3svr)
	})

	t.Run("TestNewBlockFilterIntegrity", func(t *testing.T) {
		newBlockFilterIntegrity(t, web3svr)
	})

	t.Run("TestFilterChangesIntegrity", func(t *testing.T) {
		filterChangesIntegrity(t, web3svr)
	})

	t.Run("TestFilterLogsIntegrity", func(t *testing.T) {
		filterLogsIntegrity(t, web3svr)
	})

	t.Run("TestNetworkIDIntegrity", func(t *testing.T) {
		networkIDIntegrity(t, web3svr)
	})

	t.Run("TestEthAccountsIntegrity", func(t *testing.T) {
		ethAccountsIntegrity(t, web3svr)
	})

	t.Run("TestWeb3StakingIntegrity", func(t *testing.T) {
		web3StakingIntegrity(t, web3svr)
	})

	t.Run("TestSendRawTransactionIntegrity", func(t *testing.T) {
		sendRawTransactionIntegrity(t, web3svr)
	})

	t.Run("TestEstimateGasIntegrity", func(t *testing.T) {
		estimateGasIntegrity(t, web3svr, bc, dao, actPool)
	})

	t.Run("TestGetCodeIntegrity", func(t *testing.T) {
		codeIntegrity(t, web3svr, bc, dao, actPool)
	})

	t.Run("TestGetStorageAtIntegrity", func(t *testing.T) {
		storageAtIntegrity(t, web3svr, bc, dao, actPool)
	})
}

func setupTestServer() (*ServerV2, blockchain.Blockchain, blockdao.BlockDAO, actpool.ActPool, func()) {
	cfg := newConfig()
	config.SetEVMNetworkID(_evmNetworkID)
	svr, bc, dao, _, _, actPool, bfIndexFile, _ := createServerV2(cfg, false)
	return svr, bc, dao, actPool, func() {
		testutil.CleanupPath(bfIndexFile)
	}
}

func serveTestHttp(web3svr *Web3Server, method string, paramData string) (string, error) {
	req, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(fmt.Sprintf(`[{"jsonrpc":"2.0",%s,%s,"id":1}]`, method, paramData)))
	resp := httptest.NewRecorder()
	web3svr.ServeHTTP(resp, req)
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
	return string(body), nil
	// var web3Resqs []web3Response
	// err := json.Unmarshal(body, &web3Resqs)
	// return web3Resqs, err
}

func TestLocalAPICacheIntegrity(t *testing.T) {
	require := require.New(t)
	testKey, testData := strconv.Itoa(rand.Int()), []byte(strconv.Itoa(rand.Int()))
	cacheLocal := newAPICache(1*time.Second, "")
	_, exist := cacheLocal.Get(testKey)
	require.False(exist)
	err := cacheLocal.Set(testKey, testData)
	require.NoError(err)
	data, _ := cacheLocal.Get(testKey)
	require.Equal(data, testData)
	cacheLocal.Del(testKey)
	_, exist = cacheLocal.Get(testKey)
	require.False(exist)
}

func gasPriceIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	resp, err := serveTestHttp(web3svr, `"method":"eth_gasPrice"`, `"params":[]`)
	require.NoError(err)
	t.Log(resp)
	t.Logf("%T", resp)
	t.Log(uint64ToHex(1000000000000))
	require.JSONEq(fmt.Sprintf(`
		{
			"jsonrpc":"2.0",
			"id":1,
			"result":"%s"
		 }
		`, uint64ToHex(1000000000000)),
		string(resp))
	// ret, ok := resp[0].result.(string)
	// require.True(ok)
	// require.Equal(uint64ToHex(1000000000000), ret)
}

func chainIDIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	resp, err := serveTestHttp(web3svr, `"method":"eth_chainId"`, `"params":[]`)
	require.NoError(err)
	require.JSONEq(fmt.Sprintf(`
		{
			"jsonrpc":"2.0",
			"id":1,
			"result":"%s"
		}
		`, uint64ToHex(1)),
		string(resp))
	// ret, ok := resp[0].result.(string)
	// require.True(ok)
	// require.Equal(uint64ToHex(1), ret)
}

func blockNumberIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	resp, err := serveTestHttp(web3svr, `"method":"eth_blockNumber"`, `"params":[]`)
	require.NoError(err)
	require.JSONEq(fmt.Sprintf(`
		{
			"jsonrpc":"2.0",
			"id":1,
			"result":"%s"
		}
		`, uint64ToHex(4)),
		string(resp))
	// ret, ok := resp[0].result.(string)
	// require.True(ok)
	// require.Equal(uint64ToHex(4), ret)
}

func balanceIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	resp, err := serveTestHttp(web3svr, `"method":"eth_getBalance"`, `"params": ["0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", 1]`)
	require.NoError(err)
	ans, _ := new(big.Int).SetString("9999999999999999999999999991", 10)
	require.JSONEq(fmt.Sprintf(`
		{
			"jsonrpc":"2.0",
			"id":1,
			"result":"%s"
		}
		`, "0x"+fmt.Sprintf("%x", ans)),
		string(resp))
	// ret, ok := resp[0].result.(string)
	// require.True(ok)
	// ans, _ := new(big.Int).SetString("9999999999999999999999999991", 10)
	// require.Equal("0x"+fmt.Sprintf("%x", ans), ret)
}

func blockByNumberIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestWeb3Server()
	defer cleanCallback()

	testData := []struct {
		data     string
		expected int
	}{
		{`"params": ["1", true]`, 1},
		{`"params": ["1", false]`, 2},
		{`"params": ["10", false]`, 0},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			resp, err := serveTestHttp(web3svr, `"method":"eth_getBlockByNumber"`, v.data)
			require.NoError(err)

			if v.expected == 0 {
				require.Nil(resp[0].result)
				return
			}
			transactions, ok := resp[0].result.(map[string]interface{})["transactions"]
			require.True(ok)
			require.Equal(v.expected, len(transactions.([]interface{})))
		})
	}
}

func transactionCountIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestWeb3Server()
	defer cleanCallback()

	testData := []struct {
		data     string
		expected int
	}{
		{`"params": ["0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", "0x1"]`, 2},
		{`"params": ["0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", "pending"]`, 2},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			resp, err := serveTestHttp(web3svr, `"method":"eth_getTransactionCount"`, v.data)

			require.NoError(err)
			ret, ok := resp[0].result.(string)
			require.True(ok)
			require.Equal(uint64ToHex(uint64(v.expected)), ret)
		})
	}
}

func callIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	testData := []struct {
		data     string
		expected int
	}{
		{
			`"params": [{
					"from":     "",
					"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
					"gas":      "0x4e20",
					"gasPrice": "0xe8d4a51000",
					"value":    "0x1",
					"data":     "0x1"},
				1]`,
			1,
		},
		{
			`"params": [{
					"from":     "",
					"to":       "0xb1f8e55c7f64d203c1400b9d8555d050f94adf39",
					"gas":      "0x4e20",
					"gasPrice": "0xe8d4a51000",
					"value":    "0x1",
					"data":     "0x1"},
				1]`,
			0,
		},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			resp, err := serveTestHttp(web3svr, `"method":"eth_call"`, v.data)
			require.NoError(err)
			if v.expected == 0 {
				require.Nil(resp[0].result)
				return
			}
			ret, ok := resp[0].result.(string)
			require.True(ok)
			require.Equal("0x", ret)
		})
	}
}

func nodeInfoIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	resp, err := serveTestHttp(web3svr, `"method":"web3_clientVersion"`, `"params":[]`)
	require.NoError(err)
	ret, ok := resp[0].result.(string)
	require.True(ok)
	require.Equal("NoBuildInfo/NoBuildInfo", ret)
}

func blockTransactionCountByHashIntegrity(t *testing.T, web3svr *Web3Server, bc blockchain.Blockchain) {
	require := require.New(t)
	header, err := bc.BlockHeaderByHeight(1)
	require.NoError(err)
	blkHash := header.HashBlock()

	resp, err := serveTestHttp(web3svr,
		`"method":"eth_getBlockTransactionCountByHash"`,
		fmt.Sprintf(`"params":["0x%s", 1]`, hex.EncodeToString(blkHash[:])))
	require.NoError(err)
	ret, ok := resp[0].result.(string)
	require.True(ok)
	require.Equal(uint64ToHex(2), ret)
}

func blockByHashIntegrity(t *testing.T, web3svr *Web3Server, bc blockchain.Blockchain) {
	require := require.New(t)
	header, err := bc.BlockHeaderByHeight(1)
	require.NoError(err)
	blkHash := header.HashBlock()

	testData := []struct {
		data     string
		expected int
	}{
		{fmt.Sprintf(`"params": ["0x%s", false]`, hex.EncodeToString(blkHash[:])), 1},
		{`"params":["0xa2e8e0c9cafbe93f2b7f7c9d32534bc6fde95f2185e5f2aaa6bf7ebdf1a6610a", false]`, 0},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			resp, err := serveTestHttp(web3svr, `"method":"eth_getBlockByHash"`, v.data)
			require.NoError(err)

			if v.expected == 0 {
				require.Nil(resp[0].result)
				return
			}
			hash, ok := resp[0].result.(map[string]interface{})["hash"]
			require.True(ok)
			require.Equal("0x"+hex.EncodeToString(blkHash[:]), hash)
			transactions, ok := resp[0].result.(map[string]interface{})["transactions"]
			require.True(ok)
			require.Equal(2, len(transactions.([]interface{})))
		})
	}
}

func transactionByHashIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	testData := []struct {
		data     string
		expected int
	}{
		{fmt.Sprintf(`"params": ["0x%s", false]`, hex.EncodeToString(_transferHash1[:])), 1},
		{`"params":["0x58df1e9cb0572fea48e8ce9d9b787ae557c304657d01890f4fc5ea88a1f44c3e", false]`, 0},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			resp, err := serveTestHttp(web3svr, `"method":"eth_getTransactionByHash"`, v.data)
			require.NoError(err)

			if v.expected == 0 {
				require.Nil(resp[0].result)
				return
			}
			hash, ok := resp[0].result.(map[string]interface{})["hash"]
			require.True(ok)
			require.Equal("0x"+hex.EncodeToString(_transferHash1[:]), hash)
		})
	}
}

func logsIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	testData := []struct {
		data   string
		logLen int
	}{
		{
			`"params":[{"fromBlock":"0x1"}]`,
			4, // if deployed contract, +1
		},
		{
			// empty log
			`"params":[{"address":"0x8ce313ab12bf7aed8136ab36c623ff98c8eaad34"}]`,
			0,
		},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			resp, err := serveTestHttp(web3svr, `"method":"eth_getLogs"`, v.data)
			require.NoError(err)

			ret, ok := resp[0].result.([]interface{})
			require.True(ok)
			require.Equal(v.logLen, len(ret))
		})
	}
}

func transactionReceiptIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	testData := []struct {
		data     string
		expected int
	}{
		{fmt.Sprintf(`"params": ["0x%s", 1]`, hex.EncodeToString(_transferHash1[:])), 1},
		{`"params":["0x58df1e9cb0572fea48e8ce9d9b787ae557c304657d01890f4fc5ea88a1f44c3e", false]`, 0},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			resp, err := serveTestHttp(web3svr, `"method":"eth_getTransactionReceipt"`, v.data)
			require.NoError(err)

			if v.expected == 0 {
				require.Nil(resp[0].result)
				return
			}
			transactionHash, ok := resp[0].result.(map[string]interface{})["transactionHash"]
			require.True(ok)
			require.Equal("0x"+hex.EncodeToString(_transferHash1[:]), transactionHash)
			fromAddr, _ := ioAddrToEthAddr(identityset.Address(27).String())
			toAddr, _ := ioAddrToEthAddr(identityset.Address(30).String())
			from, ok := resp[0].result.(map[string]interface{})["from"]
			require.True(ok)
			to, ok := resp[0].result.(map[string]interface{})["to"]
			require.True(ok)
			require.Equal(strings.ToLower(fromAddr), from)
			require.Equal(toAddr, to)
			contractAddress, ok := resp[0].result.(map[string]interface{})["contractAddress"]
			require.True(ok)
			require.Nil(nil, contractAddress)
			gasUsed, ok := resp[0].result.(map[string]interface{})["gasUsed"]
			require.True(ok)
			require.Equal(uint64ToHex(10000), gasUsed)
			blockNumber, ok := resp[0].result.(map[string]interface{})["blockNumber"]
			require.True(ok)
			require.Equal(uint64ToHex(1), blockNumber)
		})
	}
}

func blockTransactionCountByNumberIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	resp, err := serveTestHttp(web3svr,
		`"method":"eth_getBlockTransactionCountByNumber"`,
		`"params":["0x1", 1]`)
	require.NoError(err)
	ret, ok := resp[0].result.(string)
	require.True(ok)
	require.Equal(uint64ToHex(2), ret)
}

func transactionByBlockHashAndIndexIntegrity(t *testing.T, web3svr *Web3Server, bc blockchain.Blockchain) {
	require := require.New(t)
	header, err := bc.BlockHeaderByHeight(1)
	require.NoError(err)
	blkHash := header.HashBlock()

	testData := []struct {
		data     string
		expected int
	}{
		{fmt.Sprintf(`"params": ["0x%s", "0x0"]`, hex.EncodeToString(blkHash[:])), 1},
		{fmt.Sprintf(`"params": ["0x%s", "0x10"]`, hex.EncodeToString(blkHash[:])), 0},
		{`"params":["0xa2e8e0c9cafbe93f2b7f7c9d32534bc6fde95f2185e5f2aaa6bf7ebdf1a6610a", "0x0"]`, 0},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			resp, err := serveTestHttp(web3svr, `"method":"eth_getTransactionByBlockHashAndIndex"`, v.data)
			require.NoError(err)

			if v.expected == 0 {
				require.Nil(resp[0].result)
				return
			}
			hash, ok := resp[0].result.(map[string]interface{})["hash"]
			require.True(ok)
			require.Equal("0x"+hex.EncodeToString(_transferHash1[:]), hash)
			fromAddr, _ := ioAddrToEthAddr(identityset.Address(27).String())
			toAddr, _ := ioAddrToEthAddr(identityset.Address(30).String())
			from, ok := resp[0].result.(map[string]interface{})["from"]
			require.True(ok)
			to, ok := resp[0].result.(map[string]interface{})["to"]
			require.True(ok)
			require.Equal(strings.ToLower(fromAddr), from)
			require.Equal(toAddr, to)

			gas, ok := resp[0].result.(map[string]interface{})["gas"]
			require.True(ok)
			require.Equal(uint64ToHex(20000), gas)
			gasPrice, ok := resp[0].result.(map[string]interface{})["gasPrice"]
			require.True(ok)
			require.Equal(uint64ToHex(0), gasPrice)
		})
	}
}

func transactionByBlockNumberAndIndexIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	testData := []struct {
		data     string
		expected int
	}{
		{`"params": ["0x1", "0x0"]`, 1},
		{`"params": ["0x1", "0x10"]`, 0},
		{`"params": ["0x10", "0x0"]`, 0},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			resp, err := serveTestHttp(web3svr, `"method":"eth_getTransactionByBlockNumberAndIndex"`, v.data)
			require.NoError(err)

			if v.expected == 0 {
				require.Nil(resp[0].result)
				return
			}
			hash, ok := resp[0].result.(map[string]interface{})["hash"]
			require.True(ok)
			require.Equal("0x"+hex.EncodeToString(_transferHash1[:]), hash)
			fromAddr, _ := ioAddrToEthAddr(identityset.Address(27).String())
			toAddr, _ := ioAddrToEthAddr(identityset.Address(30).String())
			from, ok := resp[0].result.(map[string]interface{})["from"]
			require.True(ok)
			to, ok := resp[0].result.(map[string]interface{})["to"]
			require.True(ok)
			require.Equal(strings.ToLower(fromAddr), from)
			require.Equal(toAddr, to)

			gas, ok := resp[0].result.(map[string]interface{})["gas"]
			require.True(ok)
			require.Equal(uint64ToHex(20000), gas)
			gasPrice, ok := resp[0].result.(map[string]interface{})["gasPrice"]
			require.True(ok)
			require.Equal(uint64ToHex(0), gasPrice)
		})
	}
}

func newfilterIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	resp, err := serveTestHttp(web3svr,
		`"method":"eth_newFilter"`,
		`"params":[{"fromBlock":"0x1"}]`)
	require.NoError(err)
	ret, ok := resp[0].result.(string)
	require.True(ok)
	require.Equal("0xe10f7dd489b75a36de8e246eb974827fe86a02ed19d9b475a1600cf4f935feff", ret)
}

func newBlockFilterIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	resp, err := serveTestHttp(web3svr,
		`"method":"eth_newBlockFilter"`,
		`"params":[]`)
	require.NoError(err)
	ret, ok := resp[0].result.(string)
	require.True(ok)
	// before deploy contract
	require.Equal("0x71371f8dbaefc4c96d2534163a1b461951c88520cd32bc03b5bfdfe7340bc187", ret)
}

func filterChangesIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	testDataPre := []struct {
		method   string
		data     string
		expected int
	}{
		// filter
		{`"method":"eth_newFilter"`, `"params":[{"fromBlock":"0x1"}]`, 1},
		// blockfilter
		{`"method":"eth_newBlockFilter"`, `"params":[]`, 1},
	}
	paramData := []string{}

	for i, v := range testDataPre {
		t.Run(fmt.Sprintf("%d-%d", i, len(testDataPre)-1), func(t *testing.T) {
			resp, err := serveTestHttp(web3svr, v.method, v.data)
			require.NoError(err)

			require.Equal(v.expected, len(resp))
			ret, ok := resp[0].result.(string)
			require.True(ok)
			paramData = append(paramData, fmt.Sprintf(`"params":["%s"]`, ret))
		})
	}
	require.Equal(2, len(paramData))

	testData := []struct {
		data     string
		expected int
	}{
		// filter
		{paramData[0], 4},
		{paramData[0], 0},
		// blockfilter
		{paramData[1], 1},
		{paramData[1], 0},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			resp, err := serveTestHttp(web3svr, `"method":"eth_getFilterChanges"`, v.data)
			require.NoError(err)
			ret, ok := resp[0].result.([]interface{})
			require.True(ok)
			require.Equal(v.expected, len(ret))
		})
	}
}

func filterLogsIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	resp, err := serveTestHttp(web3svr,
		`"method":"eth_newFilter"`,
		`"params":[{"fromBlock":"0x1"}]`)
	require.NoError(err)
	ret, ok := resp[0].result.(string)
	require.True(ok)

	resp, err = serveTestHttp(web3svr,
		`"method":"eth_getFilterLogs"`,
		fmt.Sprintf(`"params":["%s"]`, ret))
	require.NoError(err)
	ret1, ok := resp[0].result.([]interface{})
	require.True(ok)
	require.Equal(4, len(ret1))
}

func networkIDIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	resp, err := serveTestHttp(web3svr,
		`"method":"net_version"`,
		`"params":[]`)
	require.NoError(err)
	ret, ok := resp[0].result.(string)
	require.True(ok)
	require.Equal(fmt.Sprintf("%d", _evmNetworkID), ret)
}

func ethAccountsIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	resp, err := serveTestHttp(web3svr,
		`"method":"eth_accounts"`,
		`"params":[]`)
	require.NoError(err)
	ret, ok := resp[0].result.([]interface{})
	require.True(ok)
	require.Equal(0, len(ret))
}

func web3StakingIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	ecdsaPvk, ok := identityset.PrivateKey(28).EcdsaPrivateKey().(*ecdsa.PrivateKey)
	require.True(ok)

	type stakeData struct {
		testName         string
		stakeEncodedData []byte
	}
	testData := []stakeData{}
	toAddr, err := ioAddrToEthAddr(address.StakingProtocolAddr)
	require.NoError(err)

	// encode stake data
	act1, err := action.NewCreateStake(1, "test", "100", 7, false, []byte{}, 1000000, big.NewInt(0))
	require.NoError(err)
	data, err := act1.EncodeABIBinary()
	require.NoError(err)
	testData = append(testData, stakeData{"createStake", data})

	act2, err := action.NewDepositToStake(2, 7, "100", []byte{}, 1000000, big.NewInt(0))
	require.NoError(err)
	data2, err := act2.EncodeABIBinary()
	require.NoError(err)
	testData = append(testData, stakeData{"depositToStake", data2})

	act3, err := action.NewChangeCandidate(3, "test", 7, []byte{}, 1000000, big.NewInt(0))
	require.NoError(err)
	data3, err := act3.EncodeABIBinary()
	require.NoError(err)
	testData = append(testData, stakeData{"changeCandidate", data3})

	act4, err := action.NewUnstake(4, 7, []byte{}, 1000000, big.NewInt(0))
	require.NoError(err)
	data4, err := act4.EncodeABIBinary()
	require.NoError(err)
	testData = append(testData, stakeData{"unstake", data4})

	act5, err := action.NewWithdrawStake(5, 7, []byte{}, 1000000, big.NewInt(0))
	require.NoError(err)
	data5, err := act5.EncodeABIBinary()
	require.NoError(err)
	testData = append(testData, stakeData{"withdrawStake", data5})

	act6, err := action.NewRestake(6, 7, 7, false, []byte{}, 1000000, big.NewInt(0))
	require.NoError(err)
	data6, err := act6.EncodeABIBinary()
	require.NoError(err)
	testData = append(testData, stakeData{"restake", data6})

	act7, err := action.NewTransferStake(7, "io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza", 7, []byte{}, 1000000, big.NewInt(0))
	require.NoError(err)
	data7, err := act7.EncodeABIBinary()
	require.NoError(err)
	testData = append(testData, stakeData{"transferStake", data7})

	act8, err := action.NewCandidateRegister(
		8,
		"test",
		"io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza",
		"io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza",
		"io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza",
		"100",
		7,
		false,
		[]byte{},
		1000000,
		big.NewInt(0))
	require.NoError(err)
	data8, err := act8.EncodeABIBinary()
	require.NoError(err)
	testData = append(testData, stakeData{"candidateRegister", data8})

	act9, err := action.NewCandidateUpdate(
		9,
		"test",
		"io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza",
		"io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza",
		1000000,
		big.NewInt(0))
	require.NoError(err)
	data9, err := act9.EncodeABIBinary()
	require.NoError(err)
	testData = append(testData, stakeData{"candidateUpdate", data9})

	for i, test := range testData {
		t.Run(test.testName, func(t *testing.T) {
			// estimate gas
			gasLimit, err := estimateStakeGasIntegrity(t, web3svr, identityset.Address(28).Hex(), toAddr, test.stakeEncodedData)
			require.NoError(err)

			// create tx
			rawTx := types.NewTransaction(
				uint64(9+i),
				common.HexToAddress(toAddr),
				big.NewInt(0),
				gasLimit,
				big.NewInt(0),
				test.stakeEncodedData,
			)
			tx, err := types.SignTx(rawTx, types.NewEIP155Signer(big.NewInt(int64(_evmNetworkID))), ecdsaPvk)
			require.NoError(err)
			BinaryData, err := tx.MarshalBinary()
			require.NoError(err)

			// send tx
			resp, err := serveTestHttp(web3svr,
				`"method":"eth_sendRawTransaction"`,
				fmt.Sprintf(`"params":["%s"]`, hex.EncodeToString(BinaryData)))
			require.NoError(err)

			ret, ok := resp[0].result.(string)
			require.True(ok)
			require.Equal(64, len(util.Remove0xPrefix(ret)))
		})
	}
}

func estimateStakeGasIntegrity(t *testing.T, web3svr *Web3Server, fromAddr, toAddr string, data []byte) (uint64, error) {
	require := require.New(t)
	resp, err := serveTestHttp(web3svr,
		`"method":"eth_estimateGas"`,
		fmt.Sprintf(`"params": [{
			"from":     "%s",
			"to":       "%s",
			"gas":      "0x0",
			"gasPrice": "0x0",
			"value":    "0x0",
			"data":     "%s"},
		1]`, fromAddr, toAddr, hex.EncodeToString(data)))
	require.NoError(err)
	ret, ok := resp[0].result.(string)
	require.True(ok)
	return hexStringToNumber(ret)
}

func sendRawTransactionIntegrity(t *testing.T, web3svr *Web3Server) {
	require := require.New(t)
	resp, err := serveTestHttp(web3svr, `"method":"eth_sendRawTransaction"`, `"params":["f8600180830186a09412745fec82b585f239c01090882eb40702c32b04808025a0b0e1aab5b64d744ae01fc9f1c3e9919844a799e90c23129d611f7efe6aec8a29a0195e28d22d9b280e00d501ff63525bb76f5c87b8646c89d5d9c5485edcb1b498"]`)
	require.NoError(err)
	ret, ok := resp[0].result.(string)
	require.True(ok)
	require.Equal("0x778fd5a054e74e9055bf68ef5f9d559fa306e8ba7dee608d0a3624cca0b63b3e", ret)
}

func estimateGasIntegrity(t *testing.T, web3svr *Web3Server, bc blockchain.Blockchain, dao blockdao.BlockDAO, actPool actpool.ActPool) {
	require := require.New(t)

	// deploy a contract
	contractCode := "608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c14610057575b600080fd5b6100556004803603810190610050919061009d565b610075565b005b61005f61007f565b60405161006c91906100d9565b60405180910390f35b8060008190555050565b60008054905090565b60008135905061009781610103565b92915050565b6000602082840312156100b3576100b26100fe565b5b60006100c184828501610088565b91505092915050565b6100d3816100f4565b82525050565b60006020820190506100ee60008301846100ca565b92915050565b6000819050919050565b600080fd5b61010c816100f4565b811461011757600080fd5b5056fea2646970667358221220c86a8c4dd175f55f5732b75b721d714ceb38a835b87c6cf37cf28c790813e19064736f6c63430008070033"
	contract, _ := deployContractV2(&ServerV2{}, bc, dao, actPool, identityset.PrivateKey(13), 1, bc.TipHeight(), contractCode)

	fromAddr, _ := ioAddrToEthAddr(identityset.Address(0).String())
	toAddr, _ := ioAddrToEthAddr(identityset.Address(28).String())
	contractAddr, _ := ioAddrToEthAddr(contract)
	testData := []struct {
		input  string
		result uint64
	}{
		{
			input: fmt.Sprintf(`"params": [{
				"from":     "%s",
				"to":       "%s",
				"gas":      "0x0",
				"gasPrice": "0x0",
				"value":    "0x0",
				"data":     "0x1123123c"},
			1]`, fromAddr, toAddr),
			result: 21000,
		},
		{
			input: fmt.Sprintf(`"params": [{
			"from":     "%s",
			"to":       "%s",
			"gas":      "0x0",
			"gasPrice": "0x0",
			"value":    "0x0",
			"data":      "344933be000000000000000000000000000000000000000000000000000be497a92e9f3300000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000f8be4046fd89199906ca348bcd3822c4b250e246000000000000000000000000000000000000000000000000000000006173a15400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002000000000000000000000000a00744882684c3e4747faefd68d283ea44099d030000000000000000000000000258866edaf84d6081df17660357ab20a07d0c80"},
			1]`, fromAddr, toAddr),
			result: 36000,
		},
		{
			input: fmt.Sprintf(`"params": [{
			"from":     "%s",
			"to":       "%s",
			"gas":      "0x0",
			"gasPrice": "0x0",
			"value":    "0x0",
			"data":     "0x6d4ce63c"},
		1]`, fromAddr, contractAddr),
			result: 21000,
		},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			resp, err := serveTestHttp(web3svr, `"method":"eth_estimateGas"`, v.input)
			require.NoError(err)
			ret, ok := resp[0].result.(string)
			require.True(ok)
			require.Equal(uint64ToHex(v.result), ret)
		})
	}
}

func codeIntegrity(t *testing.T, web3svr *Web3Server, bc blockchain.Blockchain, dao blockdao.BlockDAO, actPool actpool.ActPool) {
	require := require.New(t)
	// deploy a contract
	contractCode := "608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c14610057575b600080fd5b6100556004803603810190610050919061009d565b610075565b005b61005f61007f565b60405161006c91906100d9565b60405180910390f35b8060008190555050565b60008054905090565b60008135905061009781610103565b92915050565b6000602082840312156100b3576100b26100fe565b5b60006100c184828501610088565b91505092915050565b6100d3816100f4565b82525050565b60006020820190506100ee60008301846100ca565b92915050565b6000819050919050565b600080fd5b61010c816100f4565b811461011757600080fd5b5056fea2646970667358221220c86a8c4dd175f55f5732b75b721d714ceb38a835b87c6cf37cf28c790813e19064736f6c63430008070033"
	contract, _ := deployContractV2(&ServerV2{}, bc, dao, actPool, identityset.PrivateKey(13), 1, bc.TipHeight(), contractCode)
	contractAddr, _ := ioAddrToEthAddr(contract)

	resp, err := serveTestHttp(web3svr, `"method":"eth_getCode"`, fmt.Sprintf(`"params":["%s", 1]`, contractAddr))
	require.NoError(err)
	ret, ok := resp[0].result.(string)
	require.True(ok)
	require.Contains(contractCode, util.Remove0xPrefix(ret))
}

func storageAtIntegrity(t *testing.T, web3svr *Web3Server, bc blockchain.Blockchain, dao blockdao.BlockDAO, actPool actpool.ActPool) {
	require := require.New(t)
	// deploy a contract
	contractCode := "608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c14610057575b600080fd5b6100556004803603810190610050919061009d565b610075565b005b61005f61007f565b60405161006c91906100d9565b60405180910390f35b8060008190555050565b60008054905090565b60008135905061009781610103565b92915050565b6000602082840312156100b3576100b26100fe565b5b60006100c184828501610088565b91505092915050565b6100d3816100f4565b82525050565b60006020820190506100ee60008301846100ca565b92915050565b6000819050919050565b600080fd5b61010c816100f4565b811461011757600080fd5b5056fea2646970667358221220c86a8c4dd175f55f5732b75b721d714ceb38a835b87c6cf37cf28c790813e19064736f6c63430008070033"
	contract, _ := deployContractV2(&ServerV2{}, bc, dao, actPool, identityset.PrivateKey(13), 1, bc.TipHeight(), contractCode)
	contractAddr, _ := ioAddrToEthAddr(contract)

	testData := []struct {
		data     string
		expected int
	}{
		{fmt.Sprintf(`"params": ["%s", "0x0"]`, contractAddr), 1},
		{`"params": [1]`, 0},
		{`"params":["TEST", "TEST"]`, 0},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			resp, err := serveTestHttp(web3svr, `"method":"eth_getStorageAt"`, v.data)
			require.NoError(err)

			if v.expected == 0 {
				require.Nil(resp[0].result)
				return
			}
			ret, ok := resp[0].result.(string)
			require.True(ok)
			// the value of any contract at pos0 is be "0x0000000000000000000000000000000000000000000000000000000000000000"
			require.Equal("0x0000000000000000000000000000000000000000000000000000000000000000", ret)
		})
	}
}
