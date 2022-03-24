// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/util"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	_evmNetworkID uint32 = 1
)

func TestGasPriceIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	ret, _ := svr.web3Server.gasPrice()
	require.Equal(uint64ToHex(1000000000000), ret)
}

func TestGetChainIDIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	ret, _ := svr.web3Server.getChainID()
	require.Equal(uint64ToHex(1), ret)
}

func TestGetBlockNumberIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	ret, _ := svr.web3Server.getBlockNumber()
	require.Equal(uint64ToHex(4), ret)
}

func TestGetBlockByNumberIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := []struct {
		data     string
		expected int
	}{
		{`{"params": ["1", true]}`, 1},
		{`{"params": ["1", false]}`, 2},
		{`{"params": ["10", false]}`, 0},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			data := gjson.Parse(v.data)
			ret, err := svr.web3Server.getBlockByNumber(&data)
			require.NoError(err)
			if v.expected == 0 {
				require.Nil(ret)
				return
			}
			blk, ok := ret.(blockObject)
			require.True(ok)
			require.Equal(len(blk.Transactions), v.expected)
		})
	}
}

func TestGetBalanceIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := gjson.Parse(`{"params": ["0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", 1]}`)
	ret, _ := svr.web3Server.getBalance(&testData)
	ans, _ := new(big.Int).SetString("9999999999999999999999999991", 10)
	require.Equal("0x"+fmt.Sprintf("%x", ans), ret)
}

func TestGetTransactionCountIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := []struct {
		data     string
		expected int
	}{
		{`{"params": ["0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", "0x1"]}`, 2},
		{`{"params": ["0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", "pending"]}`, 2},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			data := gjson.Parse(v.data)
			ret, _ := svr.web3Server.getTransactionCount(&data)
			require.Equal(uint64ToHex(uint64(v.expected)), ret)
		})
	}
}

func TestCallIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := []struct {
		data string
	}{
		{
			`{"params": [{
				"from":     "",
				"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
				"gas":      "0x4e20",
				"gasPrice": "0xe8d4a51000",
				"value":    "0x1",
				"data":     "0x1"},
			1]}`,
		},
		{
			`{"params": [{
				"from":     "",
				"to":       "0xb1f8e55c7f64d203c1400b9d8555d050f94adf39",
				"gas":      "0x4e20",
				"gasPrice": "0xe8d4a51000",
				"value":    "0x1",
				"data":     "0x1"},
			1]}`,
		},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			data := gjson.Parse(v.data)
			_, err := svr.web3Server.call(&data)
			require.NoError(err)
		})
	}
}

func TestEstimateGasIntegrity(t *testing.T) {
	require := require.New(t)
	svr, bc, dao, actPool, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	// deploy a contract
	contractCode := "608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c14610057575b600080fd5b6100556004803603810190610050919061009d565b610075565b005b61005f61007f565b60405161006c91906100d9565b60405180910390f35b8060008190555050565b60008054905090565b60008135905061009781610103565b92915050565b6000602082840312156100b3576100b26100fe565b5b60006100c184828501610088565b91505092915050565b6100d3816100f4565b82525050565b60006020820190506100ee60008301846100ca565b92915050565b6000819050919050565b600080fd5b61010c816100f4565b811461011757600080fd5b5056fea2646970667358221220c86a8c4dd175f55f5732b75b721d714ceb38a835b87c6cf37cf28c790813e19064736f6c63430008070033"
	contract, _ := deployContractV2(svr, bc, dao, actPool, identityset.PrivateKey(13), 1, bc.TipHeight(), contractCode)

	fromAddr, _ := ioAddrToEthAddr(identityset.Address(0).String())
	toAddr, _ := ioAddrToEthAddr(identityset.Address(28).String())
	contractAddr, _ := ioAddrToEthAddr(contract)
	testData := []struct {
		input  string
		result uint64
	}{
		{
			input: fmt.Sprintf(`{"params": [{
					"from":     "%s",
					"to":       "%s",
					"gas":      "0x0",
					"gasPrice": "0x0",
					"value":    "0x0",
					"data":     "0x1123123c"},
				1]}`, fromAddr, toAddr),
			result: 21000,
		},
		{
			input: fmt.Sprintf(`{"params": [{
				"from":     "%s",
				"to":       "%s",
				"gas":      "0x0",
				"gasPrice": "0x0",
				"value":    "0x0",
				"data":      "344933be000000000000000000000000000000000000000000000000000be497a92e9f3300000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000f8be4046fd89199906ca348bcd3822c4b250e246000000000000000000000000000000000000000000000000000000006173a15400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002000000000000000000000000a00744882684c3e4747faefd68d283ea44099d030000000000000000000000000258866edaf84d6081df17660357ab20a07d0c80"},
				1]}`, fromAddr, toAddr),
			result: 36000,
		},
		{
			input: fmt.Sprintf(`{"params": [{
				"from":     "%s",
				"to":       "%s",
				"gas":      "0x0",
				"gasPrice": "0x0",
				"value":    "0x0",
				"data":     "0x6d4ce63c"},
			1]}`, fromAddr, contractAddr),
			result: 21000,
		},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			input := gjson.Parse(v.input)
			ret, err := svr.web3Server.estimateGas(&input)
			require.NoError(err)
			require.Equal(ret, uint64ToHex(v.result))
		})
	}
}

func TestSendRawTransactionIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := gjson.Parse(`{"params": ["f8600180830186a09412745fec82b585f239c01090882eb40702c32b04808025a0b0e1aab5b64d744ae01fc9f1c3e9919844a799e90c23129d611f7efe6aec8a29a0195e28d22d9b280e00d501ff63525bb76f5c87b8646c89d5d9c5485edcb1b498"]}`)
	res, _ := svr.web3Server.sendRawTransaction(&testData)
	require.Equal("0x778fd5a054e74e9055bf68ef5f9d559fa306e8ba7dee608d0a3624cca0b63b3e", res)
}

func TestGetCodeIntegrity(t *testing.T) {
	require := require.New(t)
	svr, bc, dao, actPool, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	// deploy a contract
	contractCode := "608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c14610057575b600080fd5b6100556004803603810190610050919061009d565b610075565b005b61005f61007f565b60405161006c91906100d9565b60405180910390f35b8060008190555050565b60008054905090565b60008135905061009781610103565b92915050565b6000602082840312156100b3576100b26100fe565b5b60006100c184828501610088565b91505092915050565b6100d3816100f4565b82525050565b60006020820190506100ee60008301846100ca565b92915050565b6000819050919050565b600080fd5b61010c816100f4565b811461011757600080fd5b5056fea2646970667358221220c86a8c4dd175f55f5732b75b721d714ceb38a835b87c6cf37cf28c790813e19064736f6c63430008070033"
	contract, _ := deployContractV2(svr, bc, dao, actPool, identityset.PrivateKey(13), 1, bc.TipHeight(), contractCode)

	contractAddr, _ := ioAddrToEthAddr(contract)
	testData := gjson.Parse(fmt.Sprintf(`{"params": ["%s", 1]}`, contractAddr))
	ret, _ := svr.web3Server.getCode(&testData)
	require.Contains(contractCode, util.Remove0xPrefix(ret.(string)))
}

func TestGetNodeInfoIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	_, err := svr.web3Server.getNodeInfo()
	require.NoError(err)
}

func TestGetBlockTransactionCountByHashIntegrity(t *testing.T) {
	require := require.New(t)
	svr, bc, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	header, err := bc.BlockHeaderByHeight(1)
	require.NoError(err)
	blkHash := header.HashBlock()
	testData := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", 1]}`, hex.EncodeToString(blkHash[:])))
	ret, err := svr.web3Server.getBlockTransactionCountByHash(&testData)
	require.NoError(err)
	require.Equal(uint64ToHex(2), ret)
}

func TestGetBlockByHashIntegrity(t *testing.T) {
	require := require.New(t)
	svr, bc, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	header, _ := bc.BlockHeaderByHeight(1)
	blkHash := header.HashBlock()

	testData := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", false]}`, hex.EncodeToString(blkHash[:])))
	ret, err := svr.web3Server.getBlockByHash(&testData)
	require.NoError(err)
	ans := ret.(blockObject)
	require.Equal("0x"+hex.EncodeToString(blkHash[:]), ans.Hash)
	require.Equal(2, len(ans.Transactions))

	testData2 := gjson.Parse(`{"params":["0xa2e8e0c9cafbe93f2b7f7c9d32534bc6fde95f2185e5f2aaa6bf7ebdf1a6610a", false]}`)
	ret, err = svr.web3Server.getBlockByHash(&testData2)
	require.NoError(err)
	require.Nil(ret)
}

func TestGetTransactionByHashIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", false]}`, hex.EncodeToString(transferHash1[:])))
	ret, err := svr.web3Server.getTransactionByHash(&testData)
	require.NoError(err)
	require.Equal("0x"+hex.EncodeToString(transferHash1[:]), ret.(transactionObject).Hash)

	testData2 := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", false]}`, "0x58df1e9cb0572fea48e8ce9d9b787ae557c304657d01890f4fc5ea88a1f44c3e"))
	ret, err = svr.web3Server.getTransactionByHash(&testData2)
	require.NoError(err)
	require.Nil(ret)
}

func TestGetLogsIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := []struct {
		data   *filterObject
		logLen int
	}{
		{
			&filterObject{FromBlock: "0x1"},
			4,
		},
		{
			// empty log
			&filterObject{Address: []string{"0x8ce313ab12bf7aed8136ab36c623ff98c8eaad34"}},
			0,
		},
	}
	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			ret, err := svr.web3Server.getLogs(v.data)
			require.NoError(err)
			require.Equal(len(ret.([]logsObject)), v.logLen)
		})
	}
}

func TestGetTransactionReceiptIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", 1]}`, hex.EncodeToString(transferHash1[:])))
	ret, err := svr.web3Server.getTransactionReceipt(&testData)
	require.NoError(err)
	ans, ok := ret.(receiptObject)
	require.True(ok)
	require.Equal(ans.TransactionHash, "0x"+hex.EncodeToString(transferHash1[:]))
	fromAddr, _ := ioAddrToEthAddr(identityset.Address(27).String())
	toAddr, _ := ioAddrToEthAddr(identityset.Address(30).String())
	require.Equal(ans.From, fromAddr)
	require.Equal(*ans.To, toAddr)
	require.Nil(nil, ans.ContractAddress)
	require.Equal(uint64ToHex(10000), ans.GasUsed)

	testData2 := gjson.Parse(`{"params": ["0x58df1e9cb0572fea48e8ce9d9b787ae557c304657d01890f4fc5ea88a1f44c3e", 1]}`)
	ret, err = svr.web3Server.getTransactionReceipt(&testData2)
	require.NoError(err)
	require.Nil(ret)
}

func TestGetBlockTransactionCountByNumberIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, _, _, _, _, _, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(bfIndexFile)
	}()

	testData := gjson.Parse(`{"params": ["0x1", 1]}`)
	ret, err := svr.web3Server.getBlockTransactionCountByNumber(&testData)
	require.NoError(err)
	require.Equal(ret, uint64ToHex(2))
}

func TestGetTransactionByBlockHashAndIndexIntegrity(t *testing.T) {
	require := require.New(t)
	svr, bc, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	header, _ := bc.BlockHeaderByHeight(1)
	blkHash := header.HashBlock()

	testData := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", "0x0"]}`, hex.EncodeToString(blkHash[:])))
	ret, err := svr.web3Server.getTransactionByBlockHashAndIndex(&testData)
	ans := ret.(transactionObject)
	require.NoError(err)
	require.Equal(ans.Hash, "0x"+hex.EncodeToString(transferHash1[:]))
	fromAddr, _ := ioAddrToEthAddr(identityset.Address(27).String())
	toAddr, _ := ioAddrToEthAddr(identityset.Address(30).String())
	require.Equal(ans.From, fromAddr)
	require.Equal(*ans.To, toAddr)
	require.Equal(ans.Gas, uint64ToHex(20000))
	require.Equal(ans.GasPrice, uint64ToHex(0))

	testData2 := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", "0x10"]}`, hex.EncodeToString(blkHash[:])))
	_, err = svr.web3Server.getTransactionByBlockHashAndIndex(&testData2)
	require.Error(err)

	testData3 := gjson.Parse(fmt.Sprintf(`{"params":["0x%s", "0x0"]}`, "0xa2e8e0c9cafbe93f2b7f7c9d32534bc6fde95f2185e5f2aaa6bf7ebdf1a6610a"))
	ret, err = svr.web3Server.getTransactionByBlockHashAndIndex(&testData3)
	require.NoError(err)
	require.Nil(ret)
}

func TestGetTransactionByBlockNumberAndIndexIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := gjson.Parse(`{"params": ["0x1", "0x0"]}`)
	ret, err := svr.web3Server.getTransactionByBlockNumberAndIndex(&testData)
	ans := ret.(transactionObject)
	require.NoError(err)
	require.Equal(ans.Hash, "0x"+hex.EncodeToString(transferHash1[:]))
	fromAddr, _ := ioAddrToEthAddr(identityset.Address(27).String())
	toAddr, _ := ioAddrToEthAddr(identityset.Address(30).String())
	require.Equal(ans.From, fromAddr)
	require.Equal(*ans.To, toAddr)
	require.Equal(ans.Gas, uint64ToHex(20000))
	require.Equal(ans.GasPrice, uint64ToHex(0))

	testData2 := gjson.Parse(`{"params": ["0x1", "0x10"]}`)
	_, err = svr.web3Server.getTransactionByBlockNumberAndIndex(&testData2)
	require.Error(err)

	testData3 := gjson.Parse(`{"params": ["0x10", "0x0"]}`)
	ret, err = svr.web3Server.getTransactionByBlockNumberAndIndex(&testData3)
	require.NoError(err)
	require.Nil(ret)
}

func TestNewfilterIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := &filterObject{FromBlock: "0x1"}
	ret, err := svr.web3Server.newFilter(testData)
	require.NoError(err)
	require.Equal(ret, "0xe10f7dd489b75a36de8e246eb974827fe86a02ed19d9b475a1600cf4f935feff")
}

func TestNewBlockFilterIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	ret, err := svr.web3Server.newBlockFilter()
	require.NoError(err)
	require.Equal("0x71371f8dbaefc4c96d2534163a1b461951c88520cd32bc03b5bfdfe7340bc187", ret)
}

func TestGetFilterChangesIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	// filter
	filterReq := &filterObject{FromBlock: "0x1"}
	filterID1, _ := svr.web3Server.newFilter(filterReq)
	filterID1Req := gjson.Parse(fmt.Sprintf(`{"params":["%s"]}`, filterID1.(string)))
	ret, err := svr.web3Server.getFilterChanges(&filterID1Req)
	require.NoError(err)
	require.Equal(len(ret.([]logsObject)), 4)
	// request again after last rolling
	ret, err = svr.web3Server.getFilterChanges(&filterID1Req)
	require.NoError(err)
	require.Equal(len(ret.([]logsObject)), 0)

	// blockfilter
	filterID2, _ := svr.web3Server.newBlockFilter()
	filterID2Req := gjson.Parse(fmt.Sprintf(`{"params":["%s"]}`, filterID2.(string)))
	ret2, err := svr.web3Server.getFilterChanges(&filterID2Req)
	require.NoError(err)
	require.Equal(1, len(ret2.([]string)))
	ret3, err := svr.web3Server.getFilterChanges(&filterID2Req)
	require.NoError(err)
	require.Equal(0, len(ret3.([]string)))

}

func TestGetFilterLogsIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	filterReq := &filterObject{FromBlock: "0x1"}
	filterID, _ := svr.web3Server.newFilter(filterReq)
	filterIDReq := gjson.Parse(fmt.Sprintf(`{"params":["%s"]}`, filterID.(string)))
	ret, err := svr.web3Server.getFilterLogs(&filterIDReq)
	require.NoError(err)
	require.Equal(len(ret.([]logsObject)), 4)
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

func TestGetStorageAtIntegrity(t *testing.T) {
	require := require.New(t)
	svr, bc, dao, actPool, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	// deploy a contract
	contractCode := "608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c14610057575b600080fd5b6100556004803603810190610050919061009d565b610075565b005b61005f61007f565b60405161006c91906100d9565b60405180910390f35b8060008190555050565b60008054905090565b60008135905061009781610103565b92915050565b6000602082840312156100b3576100b26100fe565b5b60006100c184828501610088565b91505092915050565b6100d3816100f4565b82525050565b60006020820190506100ee60008301846100ca565b92915050565b6000819050919050565b600080fd5b61010c816100f4565b811461011757600080fd5b5056fea2646970667358221220c86a8c4dd175f55f5732b75b721d714ceb38a835b87c6cf37cf28c790813e19064736f6c63430008070033"
	contract, _ := deployContractV2(svr, bc, dao, actPool, identityset.PrivateKey(13), 1, bc.TipHeight(), contractCode)

	contractAddr, _ := ioAddrToEthAddr(contract)
	testData := gjson.Parse(fmt.Sprintf(`{"params": ["%s", "0x0"]}`, contractAddr))
	ret, err := svr.web3Server.getStorageAt(&testData)
	require.NoError(err)
	// the value of any contract at pos0 is be "0x0000000000000000000000000000000000000000000000000000000000000000"
	require.Equal("0x0000000000000000000000000000000000000000000000000000000000000000", ret)

	failData := []gjson.Result{
		gjson.Parse(`{"params": [1]}`),
		gjson.Parse(`{"params": ["TEST", "TEST"]}`),
	}
	for _, v := range failData {
		_, err := svr.web3Server.getStorageAt(&v)
		require.Error(err)
	}
}

func TestGetNetworkIDIntegrity(t *testing.T) {
	require := require.New(t)
	svr, _, _, _, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	res, _ := svr.web3Server.getNetworkID()
	require.Equal(fmt.Sprintf("%d", _evmNetworkID), res)
}

func setupTestServer(t *testing.T) (*ServerV2, blockchain.Blockchain, blockdao.BlockDAO, actpool.ActPool, func()) {
	cfg := newConfig(t)
	config.SetEVMNetworkID(_evmNetworkID)
	svr, bc, dao, _, _, actPool, bfIndexFile, _ := createServerV2(cfg, false)
	return svr, bc, dao, actPool, func() {
		testutil.CleanupPath(bfIndexFile)
	}
}

func TestEthAccountsIntegrity(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, _, _, _, _, _, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(bfIndexFile)
	}()
	res, _ := svr.web3Server.ethAccounts()
	require.Equal(0, len(res.([]string)))
}
