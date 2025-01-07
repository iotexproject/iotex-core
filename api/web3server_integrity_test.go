// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/util"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
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
	handler := newHTTPHandler(NewWeb3Handler(svr.core, "", _defaultBatchRequestLimit))

	// send request
	t.Run("eth_gasPrice", func(t *testing.T) {
		gasPrice(t, handler)
	})

	t.Run("eth_maxPriorityFeePerGas", func(t *testing.T) {
		maxPriorityFee(t, handler)
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

	t.Run("eth_feeHistory", func(t *testing.T) {
		feeHistory(t, handler, bc, dao, actPool)
	})

	t.Run("eth_blobBaseFee", func(t *testing.T) {
		blobBaseFee(t, handler, bc, dao, actPool)
	})
}

func setupTestServer() (*ServerV2, blockchain.Blockchain, blockdao.BlockDAO, actpool.ActPool, func()) {
	cfg := newConfig()
	cfg.chain.EVMNetworkID = _evmNetworkID
	cfg.api.HTTPPort = testutil.RandomPort()
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

func gasPrice(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "eth_gasPrice", "[]")
	actual, ok := result.(string)
	require.True(ok)
	require.Equal(uint64ToHex(1000000000000), actual)
}

func maxPriorityFee(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "eth_maxPriorityFeePerGas", "[]")
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
		{`["1", true]`, 2},
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
	result := serveTestHTTP(require, handler, "eth_getBalance", `["0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36"]`)
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
			  }]`,
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
			   }]`,
			0,
		},
	} {
		result := serveTestHTTP(require, handler, "eth_call", test.params)
		if test.expected == 0 {
			require.Nil(result)
			continue
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
	require := require.New(t)
	header, err := bc.BlockHeaderByHeight(1)
	require.NoError(err)
	blkHash := header.HashBlock()
	result := serveTestHTTP(require, handler, "eth_getBlockTransactionCountByHash",
		fmt.Sprintf(`["0x%s", 1]`, hex.EncodeToString(blkHash[:])))
	actual, ok := result.(string)
	require.True(ok)
	require.Equal(uint64ToHex(2), actual)
}

func getBlockByHash(t *testing.T, handler *hTTPHandler, bc blockchain.Blockchain) {
	require := require.New(t)
	header, err := bc.BlockHeaderByHeight(1)
	require.NoError(err)
	blkHash := header.HashBlock()

	for _, test := range []struct {
		params   string
		expected int
	}{
		{fmt.Sprintf(`["0x%s", false]`, hex.EncodeToString(blkHash[:])), 1},
		{`["0xa2e8e0c9cafbe93f2b7f7c9d32534bc6fde95f2185e5f2aaa6bf7ebdf1a6610a", false]`, 0},
	} {
		result := serveTestHTTP(require, handler, "eth_getBlockByHash", test.params)
		if test.expected == 0 {
			require.Nil(result)
			continue
		}
		hash, ok := result.(map[string]interface{})["hash"]
		require.True(ok)
		require.Equal("0x"+hex.EncodeToString(blkHash[:]), hash)
		transactions, ok := result.(map[string]interface{})["transactions"]
		require.True(ok)
		require.Equal(2, len(transactions.([]interface{})))
	}
}

func getTransactionByHash(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	for _, test := range []struct {
		params   string
		expected int
	}{
		{fmt.Sprintf(`["0x%s", false]`, hex.EncodeToString(_transferHash1[:])), 1},
		{`["0x58df1e9cb0572fea48e8ce9d9b787ae557c304657d01890f4fc5ea88a1f44c3e", false]`, 0},
	} {
		result := serveTestHTTP(require, handler, "eth_getTransactionByHash", test.params)
		if test.expected == 0 {
			require.Nil(result)
			continue
		}
		hash, ok := result.(map[string]interface{})["hash"]
		require.True(ok)
		require.Equal("0x"+hex.EncodeToString(_transferHash1[:]), hash)
	}
}

func getLogs(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	for _, test := range []struct {
		params   string
		expected int
	}{
		{
			`[{"fromBlock":"0x1"}]`,
			4, // if deployed contract, +1
		},
		{
			// empty log
			`[{"address":"0x8ce313ab12bf7aed8136ab36c623ff98c8eaad34"}]`,
			0,
		},
	} {
		result := serveTestHTTP(require, handler, "eth_getLogs", test.params)
		ret, ok := result.([]interface{})
		require.True(ok)
		require.Equal(test.expected, len(ret))
	}
}

func getTransactionReceipt(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	for _, test := range []struct {
		params   string
		expected int
	}{
		{fmt.Sprintf(`["0x%s", false]`, hex.EncodeToString(_transferHash1[:])), 1},
		{`["0x58df1e9cb0572fea48e8ce9d9b787ae557c304657d01890f4fc5ea88a1f44c3e", false]`, 0},
	} {
		result := serveTestHTTP(require, handler, "eth_getTransactionReceipt", test.params)
		if test.expected == 0 {
			require.Nil(result)
			continue
		}
		transactionHash, ok := result.(map[string]interface{})["transactionHash"]
		require.True(ok)
		require.Equal("0x"+hex.EncodeToString(_transferHash1[:]), transactionHash)
		fromAddr, _ := ioAddrToEthAddr(identityset.Address(27).String())
		toAddr, _ := ioAddrToEthAddr(identityset.Address(30).String())
		from, ok := result.(map[string]interface{})["from"]
		require.True(ok)
		to, ok := result.(map[string]interface{})["to"]
		require.True(ok)
		require.Equal(strings.ToLower(fromAddr), from)
		require.Equal(toAddr, to)
		contractAddress, ok := result.(map[string]interface{})["contractAddress"]
		require.True(ok)
		require.Nil(nil, contractAddress)
		gasUsed, ok := result.(map[string]interface{})["gasUsed"]
		require.True(ok)
		require.Equal(uint64ToHex(10000), gasUsed)
		blockNumber, ok := result.(map[string]interface{})["blockNumber"]
		require.True(ok)
		require.Equal(uint64ToHex(1), blockNumber)
	}
}

func getBlockTransactionCountByNumber(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "eth_getBlockTransactionCountByNumber", `["0x1"]`)
	actual, ok := result.(string)
	require.True(ok)
	require.Equal(uint64ToHex(2), actual)
}

func getTransactionByBlockHashAndIndex(t *testing.T, handler *hTTPHandler, bc blockchain.Blockchain) {
	require := require.New(t)
	header, err := bc.BlockHeaderByHeight(1)
	require.NoError(err)
	blkHash := header.HashBlock()

	for _, test := range []struct {
		params   string
		expected int
	}{
		{fmt.Sprintf(`["0x%s", "0x0"]`, hex.EncodeToString(blkHash[:])), 1},
		{fmt.Sprintf(`["0x%s", "0x10"]`, hex.EncodeToString(blkHash[:])), 0},
		{`["0xa2e8e0c9cafbe93f2b7f7c9d32534bc6fde95f2185e5f2aaa6bf7ebdf1a6610a", "0x0"]`, 0},
	} {
		result := serveTestHTTP(require, handler, "eth_getTransactionByBlockHashAndIndex", test.params)
		if test.expected == 0 {
			require.Nil(result)
			continue
		}
		hash, ok := result.(map[string]interface{})["hash"]
		require.True(ok)
		require.Equal("0x"+hex.EncodeToString(_transferHash1[:]), hash)
		fromAddr, _ := ioAddrToEthAddr(identityset.Address(27).String())
		toAddr, _ := ioAddrToEthAddr(identityset.Address(30).String())
		from, ok := result.(map[string]interface{})["from"]
		require.True(ok)
		to, ok := result.(map[string]interface{})["to"]
		require.True(ok)
		require.Equal(strings.ToLower(fromAddr), from)
		require.Equal(toAddr, to)

		gas, ok := result.(map[string]interface{})["gas"]
		require.True(ok)
		require.Equal(uint64ToHex(20000), gas)
		gasPrice, ok := result.(map[string]interface{})["gasPrice"]
		require.True(ok)
		require.Equal(uint64ToHex(0), gasPrice)
	}
}

func getTransactionByBlockNumberAndIndex(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	for _, test := range []struct {
		params   string
		expected int
	}{
		{`["0x1", "0x0"]`, 1},
		{`["0x1", "0x10"]`, 0},
		{`["0x10", "0x0"]`, 0},
	} {
		result := serveTestHTTP(require, handler, "eth_getTransactionByBlockNumberAndIndex", test.params)
		if test.expected == 0 {
			require.Nil(result)
			continue
		}
		hash, ok := result.(map[string]interface{})["hash"]
		require.True(ok)
		require.Equal("0x"+hex.EncodeToString(_transferHash1[:]), hash)
		fromAddr, _ := ioAddrToEthAddr(identityset.Address(27).String())
		toAddr, _ := ioAddrToEthAddr(identityset.Address(30).String())
		from, ok := result.(map[string]interface{})["from"]
		require.True(ok)
		to, ok := result.(map[string]interface{})["to"]
		require.True(ok)
		require.Equal(strings.ToLower(fromAddr), from)
		require.Equal(toAddr, to)

		gas, ok := result.(map[string]interface{})["gas"]
		require.True(ok)
		require.Equal(uint64ToHex(20000), gas)
		gasPrice, ok := result.(map[string]interface{})["gasPrice"]
		require.True(ok)
		require.Equal(uint64ToHex(0), gasPrice)
	}
}

func newfilter(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "eth_newFilter", `[{"fromBlock":"0x1"}]`)
	actual, ok := result.(string)
	require.True(ok)
	require.Equal("0xe10f7dd489b75a36de8e246eb974827fe86a02ed19d9b475a1600cf4f935feff", actual)
}

func newBlockFilter(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "eth_newBlockFilter", "[]")
	actual, ok := result.(string)
	require.True(ok)
	require.Equal("0x71371f8dbaefc4c96d2534163a1b461951c88520cd32bc03b5bfdfe7340bc187", actual)
}

func getFilterChanges(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)

	result := serveTestHTTP(require, handler, "eth_newFilter", `[{"fromBlock":"0x1"}]`)
	actual, ok := result.(string)
	require.True(ok)
	result1 := serveTestHTTP(require, handler, "eth_getFilterChanges", fmt.Sprintf(`["%s"]`, actual))
	actual1, ok := result1.([]interface{})
	require.True(ok)
	require.Len(actual1, 4)
	// request again after last rolling
	result1 = serveTestHTTP(require, handler, "eth_getFilterChanges", fmt.Sprintf(`["%s"]`, actual))
	actual1, ok = result1.([]interface{})
	require.True(ok)
	require.Len(actual1, 0)

	result = serveTestHTTP(require, handler, "eth_newBlockFilter", `[]`)
	actual, ok = result.(string)
	require.True(ok)
	result1 = serveTestHTTP(require, handler, "eth_getFilterChanges", fmt.Sprintf(`["%s"]`, actual))
	actual1, ok = result1.([]interface{})
	require.True(ok)
	require.Len(actual1, 1)
	// request again after last rolling
	result1 = serveTestHTTP(require, handler, "eth_getFilterChanges", fmt.Sprintf(`["%s"]`, actual))
	actual1, ok = result1.([]interface{})
	require.True(ok)
	require.Len(actual1, 0)
}

func getFilterLogs(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "eth_newFilter", `[{"fromBlock":"0x1"}]`)
	actual, ok := result.(string)
	require.True(ok)

	result1 := serveTestHTTP(require, handler, "eth_getFilterLogs", fmt.Sprintf(`["%s"]`, actual))
	actual1, ok := result1.([]interface{})
	require.True(ok)
	require.Equal(4, len(actual1))
}

func getNetworkID(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "net_version", "[]")
	actual, ok := result.(string)
	require.True(ok)
	require.Equal(fmt.Sprintf("%d", _evmNetworkID), actual)
}

func ethAccounts(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	result := serveTestHTTP(require, handler, "eth_accounts", "[]")
	actual, ok := result.([]interface{})
	require.True(ok)
	require.Equal(0, len(actual))
}

func web3Staking(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	ecdsaPvk, ok := identityset.PrivateKey(28).EcdsaPrivateKey().(*ecdsa.PrivateKey)
	require.True(ok)
	toAddr, err := ioAddrToEthAddr(address.StakingProtocolAddr)
	require.NoError(err)

	type stakeData struct {
		actType string
		data    []byte
	}
	var testDatas []stakeData

	// encode stake data
	act1, err := action.NewCreateStake("test", "100", 7, false, []byte{})
	require.NoError(err)
	data, err := act1.EthData()
	require.NoError(err)
	testDatas = append(testDatas, stakeData{"createStake", data})

	act2, err := action.NewDepositToStake(7, "100", []byte{})
	require.NoError(err)
	data2, err := act2.EthData()
	require.NoError(err)
	testDatas = append(testDatas, stakeData{"depositToStake", data2})

	act3 := action.NewChangeCandidate("test", 7, []byte{})
	require.NoError(err)
	data3, err := act3.EthData()
	require.NoError(err)
	testDatas = append(testDatas, stakeData{"changeCandidate", data3})

	act4 := action.NewUnstake(7, []byte{})
	data4, err := act4.EthData()
	require.NoError(err)
	testDatas = append(testDatas, stakeData{"unstake", data4})

	act5 := action.NewWithdrawStake(7, []byte{})
	data5, err := act5.EthData()
	require.NoError(err)
	testDatas = append(testDatas, stakeData{"withdrawStake", data5})

	act6 := action.NewRestake(7, 7, false, []byte{})
	data6, err := act6.EthData()
	require.NoError(err)
	testDatas = append(testDatas, stakeData{"restake", data6})

	act7, err := action.NewTransferStake("io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza", 7, []byte{})
	require.NoError(err)
	data7, err := act7.EthData()
	require.NoError(err)
	testDatas = append(testDatas, stakeData{"transferStake", data7})

	act8, err := action.NewCandidateRegister(
		"test",
		"io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza",
		"io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza",
		"io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza",
		"100",
		7,
		false,
		[]byte{})
	require.NoError(err)
	data8, err := act8.EthData()
	require.NoError(err)
	testDatas = append(testDatas, stakeData{"candidateRegister", data8})

	act9, err := action.NewCandidateUpdate(
		"test",
		"io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza",
		"io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza")
	require.NoError(err)
	data9, err := act9.EthData()
	require.NoError(err)
	testDatas = append(testDatas, stakeData{"candidateUpdate", data9})

	for i, test := range testDatas {
		// estimate gas
		result := serveTestHTTP(require, handler, "eth_estimateGas",
			fmt.Sprintf(`[{
				"from":     "%s",
				"to":       "%s",
				"gas":      "0x0",
				"gasPrice": "0x0",
				"value":    "0x0",
				"data":     "%s"}]`, identityset.Address(28).Hex(), toAddr, hex.EncodeToString(test.data)))
		actual, ok := result.(string)
		require.True(ok)
		gasLimit, err := hexStringToNumber(actual)
		require.NoError(err)

		// create tx
		to := common.HexToAddress(toAddr)
		rawTx := types.NewTx(&types.LegacyTx{
			Nonce:    uint64(9 + i),
			GasPrice: big.NewInt(0),
			Gas:      gasLimit,
			To:       &to,
			Value:    big.NewInt(0),
			Data:     test.data,
		})
		signer, err := action.NewEthSigner(iotextypes.Encoding_ETHEREUM_EIP155, _evmNetworkID)
		require.NoError(err)
		tx, err := types.SignTx(rawTx, signer, ecdsaPvk)
		require.NoError(err)
		BinaryData, err := tx.MarshalBinary()
		require.NoError(err)

		// send tx
		result = serveTestHTTP(require, handler, "eth_sendRawTransaction", fmt.Sprintf(`["%s"]`, hex.EncodeToString(BinaryData)))
		ret, ok := result.(string)
		require.True(ok)
		require.Equal(tx.Hash().Hex(), ret)
	}
}

func sendRawTransaction(t *testing.T, handler *hTTPHandler) {
	require := require.New(t)
	rawData := "0xf8600180830186a09412745fec82b585f239c01090882eb40702c32b04808025a0b0e1aab5b64d744ae01fc9f1c3e9919844a799e90c23129d611f7efe6aec8a29a0195e28d22d9b280e00d501ff63525bb76f5c87b8646c89d5d9c5485edcb1b498"
	tx, err := action.DecodeEtherTx(rawData)
	require.NoError(err)
	result := serveTestHTTP(require, handler, "eth_sendRawTransaction", fmt.Sprintf(`["%s"]`, rawData))
	actual, ok := result.(string)
	require.True(ok)
	require.Equal(tx.Hash().Hex(), actual)
}

func estimateGas(t *testing.T, handler *hTTPHandler, bc blockchain.Blockchain, dao blockdao.BlockDAO, actPool actpool.ActPool) {
	require := require.New(t)

	// deploy a contract
	contractCode := "608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c14610057575b600080fd5b6100556004803603810190610050919061009d565b610075565b005b61005f61007f565b60405161006c91906100d9565b60405180910390f35b8060008190555050565b60008054905090565b60008135905061009781610103565b92915050565b6000602082840312156100b3576100b26100fe565b5b60006100c184828501610088565b91505092915050565b6100d3816100f4565b82525050565b60006020820190506100ee60008301846100ca565b92915050565b6000819050919050565b600080fd5b61010c816100f4565b811461011757600080fd5b5056fea2646970667358221220c86a8c4dd175f55f5732b75b721d714ceb38a835b87c6cf37cf28c790813e19064736f6c63430008070033"
	contract, _ := deployContractV2(bc, dao, actPool, identityset.PrivateKey(13), 1, bc.TipHeight(), contractCode)

	fromAddr, _ := ioAddrToEthAddr(identityset.Address(0).String())
	toAddr, _ := ioAddrToEthAddr(identityset.Address(28).String())
	contractAddr, _ := ioAddrToEthAddr(contract)

	for _, test := range []struct {
		params   string
		expected uint64
	}{
		{
			fmt.Sprintf(`[{
				"from":     "%s",
				"to":       "%s",
				"gas":      "0x0",
				"gasPrice": "0x0",
				"value":    "0x0",
				"data":     "0x1123123c"}]`, fromAddr, toAddr),
			21000,
		},
		{
			fmt.Sprintf(`[{
			    "from":     "%s",
			    "to":       "%s",
			    "gas":      "0x0",
				"gasPrice": "0x0",
				"value":    "0x0",
				"data":      "344933be000000000000000000000000000000000000000000000000000be497a92e9f3300000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000f8be4046fd89199906ca348bcd3822c4b250e246000000000000000000000000000000000000000000000000000000006173a15400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002000000000000000000000000a00744882684c3e4747faefd68d283ea44099d030000000000000000000000000258866edaf84d6081df17660357ab20a07d0c80"}]`, fromAddr, toAddr),
			36000,
		},
		{
			fmt.Sprintf(`[{
				"from":     "%s",
				"to":       "%s",
				"gas":      "0x0",
				"gasPrice": "0x1000000000000000000",
				"value":    "0x0",
				"data":     "0x6d4ce63c"}]`, fromAddr, contractAddr),
			21000,
		},
	} {
		result := serveTestHTTP(require, handler, "eth_estimateGas", test.params)
		actual, ok := result.(string)
		require.True(ok)
		require.Equal(uint64ToHex(test.expected), actual)
	}
}

func getCode(t *testing.T, handler *hTTPHandler, bc blockchain.Blockchain, dao blockdao.BlockDAO, actPool actpool.ActPool) {
	require := require.New(t)
	// deploy a contract
	contractCode := "608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c14610057575b600080fd5b6100556004803603810190610050919061009d565b610075565b005b61005f61007f565b60405161006c91906100d9565b60405180910390f35b8060008190555050565b60008054905090565b60008135905061009781610103565b92915050565b6000602082840312156100b3576100b26100fe565b5b60006100c184828501610088565b91505092915050565b6100d3816100f4565b82525050565b60006020820190506100ee60008301846100ca565b92915050565b6000819050919050565b600080fd5b61010c816100f4565b811461011757600080fd5b5056fea2646970667358221220c86a8c4dd175f55f5732b75b721d714ceb38a835b87c6cf37cf28c790813e19064736f6c63430008070033"
	contract, _ := deployContractV2(bc, dao, actPool, identityset.PrivateKey(13), 2, bc.TipHeight(), contractCode)
	contractAddr, _ := ioAddrToEthAddr(contract)

	result := serveTestHTTP(require, handler, "eth_getCode", fmt.Sprintf(`["%s", "0x1"]`, contractAddr))
	actual, ok := result.(string)
	require.True(ok)
	require.Contains(contractCode, util.Remove0xPrefix(actual))
}

func getStorageAt(t *testing.T, handler *hTTPHandler, bc blockchain.Blockchain, dao blockdao.BlockDAO, actPool actpool.ActPool) {
	require := require.New(t)
	// deploy a contract
	contractCode := "608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c14610057575b600080fd5b6100556004803603810190610050919061009d565b610075565b005b61005f61007f565b60405161006c91906100d9565b60405180910390f35b8060008190555050565b60008054905090565b60008135905061009781610103565b92915050565b6000602082840312156100b3576100b26100fe565b5b60006100c184828501610088565b91505092915050565b6100d3816100f4565b82525050565b60006020820190506100ee60008301846100ca565b92915050565b6000819050919050565b600080fd5b61010c816100f4565b811461011757600080fd5b5056fea2646970667358221220c86a8c4dd175f55f5732b75b721d714ceb38a835b87c6cf37cf28c790813e19064736f6c63430008070033"
	contract, _ := deployContractV2(bc, dao, actPool, identityset.PrivateKey(13), 3, bc.TipHeight(), contractCode)
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
			continue
		}
		actual, ok := result.(string)
		require.True(ok)
		// the value of any contract at pos0 is be "0x0000000000000000000000000000000000000000000000000000000000000000"
		require.Equal("0x0000000000000000000000000000000000000000000000000000000000000000", actual)
	}
}

func feeHistory(t *testing.T, handler *hTTPHandler, bc blockchain.Blockchain, dao blockdao.BlockDAO, actPool actpool.ActPool) {
	require := require.New(t)
	for _, test := range []struct {
		params   string
		expected int
	}{
		{`[4, "latest", [25,75]]`, 1},
	} {
		oldnest := max(bc.TipHeight()-4+1, 1)
		result := serveTestHTTP(require, handler, "eth_feeHistory", test.params)
		if test.expected == 0 {
			require.Nil(result)
			continue
		}
		actual, err := json.Marshal(result)
		require.NoError(err)
		require.JSONEq(fmt.Sprintf(`{
    "oldestBlock": "0x%0x",
    "reward": [
      ["0x0", "0x0"],
      ["0x0", "0x0"],
      ["0x0", "0x0"],
      ["0x0", "0x0"]
    ],
    "baseFeePerGas": ["0x0","0x0","0x0","0x0","0x0"],
    "gasUsedRatio": [0,0,0,0],
    "baseFeePerBlobGas": ["0x1", "0x1", "0x1", "0x1", "0x1"],
    "blobGasUsedRatio": [0, 0, 0, 0]
  }`, oldnest), string(actual))
	}
}

func blobBaseFee(t *testing.T, handler *hTTPHandler, bc blockchain.Blockchain, dao blockdao.BlockDAO, actPool actpool.ActPool) {
	require := require.New(t)
	for _, test := range []struct {
		params   string
		expected int
	}{
		{`[]`, 1},
	} {
		result := serveTestHTTP(require, handler, "eth_blobBaseFee", test.params)
		if test.expected == 0 {
			require.Nil(result)
			continue
		}
		actual, ok := result.(string)
		require.True(ok)
		require.Equal("0x1", actual)
	}
}
