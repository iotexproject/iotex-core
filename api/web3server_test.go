package api

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/util"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/stretchr/testify/require"
)

func TestGetWeb3Reqs(t *testing.T) {
	require := require.New(t)
	testCase := []struct {
		req      *http.Request
		header   bool
		hasError bool
	}{
		// fail to decode empty data
		{
			req:      httptest.NewRequest(http.MethodPost, "http://url.com", strings.NewReader("")),
			header:   true,
			hasError: true,
		},
		//  content-type is not json
		{
			req:      httptest.NewRequest(http.MethodPut, "http://url.com", strings.NewReader("")),
			header:   false,
			hasError: true,
		},
		// missing params field
		{
			req:      httptest.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"web3_clientVersion","id":67}`)),
			header:   true,
			hasError: false,
		},
		// success
		{
			req:      httptest.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"web3_clientVersion","params":[],"id":67}`)),
			header:   true,
			hasError: false,
		},
	}

	for _, test := range testCase {
		if test.header {
			test.req.Header.Set("Content-Type", contentType)
		}
		_, err := parseWeb3Reqs(test.req)

		if test.hasError {
			require.Error(err)
		} else {
			require.NoError(err)
		}
	}
}

func TestServeHTTP(t *testing.T) {
	require := require.New(t)
	svr := &Web3Server{}

	// wrong http method
	request1, _ := http.NewRequest(http.MethodGet, "http://url.com", nil)
	response1 := getServerResp(svr, request1)
	require.Equal(response1.Result().StatusCode, http.StatusMethodNotAllowed)

	// web3 req without params
	request2, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"eth_getBalance","id":67}`))
	response2 := getServerResp(svr, request2)
	bodyBytes2, _ := ioutil.ReadAll(response2.Body)
	require.Contains(string(bodyBytes2), "invalid format")

	// missing web3 method
	request3, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"web3_foo","params":[],"id":67}`))
	response3 := getServerResp(svr, request3)
	bodyBytes3, _ := ioutil.ReadAll(response3.Body)
	require.Contains(string(bodyBytes3), "method not found")

	// single web3 req
	request4, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":67}`))
	response4 := getServerResp(svr, request4)
	bodyBytes4, _ := ioutil.ReadAll(response4.Body)
	require.Contains(string(bodyBytes4), "result")

	// multiple web3 req
	request5, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`[{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}, {"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":2}]`))
	response5 := getServerResp(svr, request5)
	bodyBytes5, _ := ioutil.ReadAll(response5.Body)
	var web3Reqs []web3Resp
	_ = json.Unmarshal(bodyBytes5, &web3Reqs)
	require.Equal(len(web3Reqs), 2)

	// web3 req without params
	request6, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"web3_clientVersion","id":67}`))
	response6 := getServerResp(svr, request6)
	bodyBytes6, _ := ioutil.ReadAll(response6.Body)
	require.Contains(string(bodyBytes6), "result")

}

func getServerResp(svr *Web3Server, req *http.Request) *httptest.ResponseRecorder {
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	svr.ServeHTTP(resp, req)
	return resp
}

func TestGasPrice(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	ret, _ := svr.web3Server.gasPrice()
	require.Equal(ret, uint64ToHex(1000000000000))
}

func TestGetChainID(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	ret, _ := svr.web3Server.getChainID()
	require.Equal(ret, uint64ToHex(1))
}

func TestGetBlockNumber(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	ret, _ := svr.web3Server.getBlockNumber()
	require.Equal(ret, uint64ToHex(4))
}

func TestGetBlockByNumber(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []struct {
		data     []interface{}
		expected int
	}{
		{[]interface{}{"1", true}, 1},
		{[]interface{}{"1", false}, 2},
		{[]interface{}{"10", false}, 0},
	}

	for _, v := range testData {
		ret, err := svr.web3Server.getBlockByNumber(v.data)
		require.NoError(err)
		if v.expected == 0 {
			require.Nil(ret)
			continue
		}
		blk, ok := ret.(blockObject)
		require.True(ok)
		require.Equal(len(blk.Transactions), v.expected)
	}
}

func TestGetBalance(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{"0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", 1}
	ret, _ := svr.web3Server.getBalance(testData)
	ans, _ := big.NewInt(0).SetString("9999999999999999999999999991", 10)
	require.Equal(ret, "0x"+fmt.Sprintf("%x", ans))
}

func TestGetTransactionCount(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{"0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", "0x1"}
	ret, _ := svr.web3Server.getTransactionCount(testData)
	require.Equal(ret, uint64ToHex(2))

	testData2 := []interface{}{"0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", "pending"}
	ret, _ = svr.web3Server.getTransactionCount(testData2)
	require.Equal(ret, uint64ToHex(2))
}

func TestCall(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{
		map[string]interface{}{
			"from":     "",
			"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
			"gas":      "0x4e20",
			"gasPrice": "0xe8d4a51000",
			"value":    "0x1",
			"data":     "0x1"},
		1}
	_, err := svr.web3Server.call(testData)
	require.NoError(err)

	testData2 := []interface{}{
		map[string]interface{}{
			"from":     "",
			"to":       "0xb1f8e55c7f64d203c1400b9d8555d050f94adf39",
			"gas":      "0x4e20",
			"gasPrice": "0xe8d4a51000",
			"value":    "0x1",
			"data":     "0x1"},
		1}
	ret, _ := svr.web3Server.call(testData2)
	require.Nil(ret)
}

func TestEstimateGas(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	// deploy a contract
	contractCode := "608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c14610057575b600080fd5b6100556004803603810190610050919061009d565b610075565b005b61005f61007f565b60405161006c91906100d9565b60405180910390f35b8060008190555050565b60008054905090565b60008135905061009781610103565b92915050565b6000602082840312156100b3576100b26100fe565b5b60006100c184828501610088565b91505092915050565b6100d3816100f4565b82525050565b60006020820190506100ee60008301846100ca565b92915050565b6000819050919050565b600080fd5b61010c816100f4565b811461011757600080fd5b5056fea2646970667358221220c86a8c4dd175f55f5732b75b721d714ceb38a835b87c6cf37cf28c790813e19064736f6c63430008070033"
	contract, _ := deployContractV2(svr, identityset.PrivateKey(13), 1, svr.core.bc.TipHeight(), contractCode)

	fromAddr, _ := ioAddrToEthAddr(identityset.Address(0).String())
	toAddr, _ := ioAddrToEthAddr(identityset.Address(28).String())
	contractAddr, _ := ioAddrToEthAddr(contract)
	testData := []struct {
		input  []interface{}
		result uint64
	}{
		{
			input: []interface{}{
				map[string]interface{}{
					"from":     fromAddr,
					"to":       toAddr,
					"gas":      "0x0",
					"gasPrice": "0x0",
					"value":    "0x0",
					"data":     "0x1123123c"},
				1},
			result: 21000,
		},
		{
			input: []interface{}{
				map[string]interface{}{
					"from":     fromAddr,
					"to":       toAddr,
					"gas":      "0x0",
					"gasPrice": "0x0",
					"value":    "0x0",
					"data":     "344933be000000000000000000000000000000000000000000000000000be497a92e9f3300000000000000000000000000000000000000000000000000000000000000a0000000000000000000000000f8be4046fd89199906ca348bcd3822c4b250e246000000000000000000000000000000000000000000000000000000006173a15400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002000000000000000000000000a00744882684c3e4747faefd68d283ea44099d030000000000000000000000000258866edaf84d6081df17660357ab20a07d0c80"},
				1},
			result: 36000,
		},
		{
			input: []interface{}{
				map[string]interface{}{
					"from":     fromAddr,
					"to":       contractAddr,
					"gas":      "0x0",
					"gasPrice": "0x0",
					"value":    "0x0",
					"data":     "0x6d4ce63c"},
				1},
			result: 21000,
		},
	}

	for _, v := range testData {
		ret, err := svr.web3Server.estimateGas(v.input)
		require.NoError(err)
		require.Equal(ret, uint64ToHex(v.result))
	}
}

func TestSendRawTransaction(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{"f8600180830186a09412745fec82b585f239c01090882eb40702c32b04808025a0b0e1aab5b64d744ae01fc9f1c3e9919844a799e90c23129d611f7efe6aec8a29a0195e28d22d9b280e00d501ff63525bb76f5c87b8646c89d5d9c5485edcb1b498"}
	res, _ := svr.web3Server.sendRawTransaction(testData)
	require.Equal(res, "0x778fd5a054e74e9055bf68ef5f9d559fa306e8ba7dee608d0a3624cca0b63b3e")
}

func TestGetCode(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()
	// deploy a contract
	contractCode := "608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c14610057575b600080fd5b6100556004803603810190610050919061009d565b610075565b005b61005f61007f565b60405161006c91906100d9565b60405180910390f35b8060008190555050565b60008054905090565b60008135905061009781610103565b92915050565b6000602082840312156100b3576100b26100fe565b5b60006100c184828501610088565b91505092915050565b6100d3816100f4565b82525050565b60006020820190506100ee60008301846100ca565b92915050565b6000819050919050565b600080fd5b61010c816100f4565b811461011757600080fd5b5056fea2646970667358221220c86a8c4dd175f55f5732b75b721d714ceb38a835b87c6cf37cf28c790813e19064736f6c63430008070033"
	contract, _ := deployContractV2(svr, identityset.PrivateKey(13), 1, svr.core.bc.TipHeight(), contractCode)

	contractAddr, _ := ioAddrToEthAddr(contract)
	testData := []interface{}{contractAddr, 1}
	ret, _ := svr.web3Server.getCode(testData)
	require.Contains(contractCode, util.Remove0xPrefix(ret.(string)))
}

func TestGetNodeInfo(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	_, err := svr.web3Server.getNodeInfo()
	require.NoError(err)
}

func TestGetBlockTransactionCountByHash(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	header, _ := svr.core.bc.BlockHeaderByHeight(1)
	blkHash := header.HashBlock()

	testData := []interface{}{"0x" + hex.EncodeToString(blkHash[:]), 1}
	ret, err := svr.web3Server.getBlockTransactionCountByHash(testData)
	require.NoError(err)
	require.Equal(ret, uint64ToHex(2))
}

func TestGetBlockByHash(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	header, _ := svr.core.bc.BlockHeaderByHeight(1)
	blkHash := header.HashBlock()

	testData := []interface{}{"0x" + hex.EncodeToString(blkHash[:]), false}
	ret, err := svr.web3Server.getBlockByHash(testData)
	require.NoError(err)
	ans, _ := ret.(blockObject)
	require.Equal(ans.Hash, "0x"+hex.EncodeToString(blkHash[:]))
	require.Equal(len(ans.Transactions), 2)

	testData2 := []interface{}{"0xa2e8e0c9cafbe93f2b7f7c9d32534bc6fde95f2185e5f2aaa6bf7ebdf1a6610a", false}
	ret, err = svr.web3Server.getBlockByHash(testData2)
	require.NoError(err)
	require.Nil(ret)
}

func TestGetTransactionByHash(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{"0x" + hex.EncodeToString(transferHash1[:]), false}
	ret, err := svr.web3Server.getTransactionByHash(testData)
	require.NoError(err)
	require.Equal(ret.(transactionObject).Hash, "0x"+hex.EncodeToString(transferHash1[:]))

	testData2 := []interface{}{"0x58df1e9cb0572fea48e8ce9d9b787ae557c304657d01890f4fc5ea88a1f44c3e", false}
	ret, err = svr.web3Server.getTransactionByHash(testData2)
	require.NoError(err)
	require.Nil(ret)
}

func TestGetLogs(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []struct {
		data   *filterObject
		logLen int
	}{
		{
			&filterObject{
				FromBlock: "0x1",
			},
			4,
		},
		{
			// empty log
			&filterObject{
				Address: []string{"0x8ce313ab12bf7aed8136ab36c623ff98c8eaad34"},
			},
			0,
		},
	}
	for _, v := range testData {
		ret, err := svr.web3Server.getLogs(v.data)
		require.NoError(err)
		require.Equal(len(ret.([]logsObject)), v.logLen)
	}
}

func TestGetTransactionReceipt(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{"0x" + hex.EncodeToString(transferHash1[:]), 1}
	ret, err := svr.web3Server.getTransactionReceipt(testData)
	require.NoError(err)
	ans := ret.(receiptObject)
	require.Equal(ans.TransactionHash, "0x"+hex.EncodeToString(transferHash1[:]))
	fromAddr, _ := ioAddrToEthAddr(identityset.Address(27).String())
	toAddr, _ := ioAddrToEthAddr(identityset.Address(30).String())
	require.Equal(ans.From, fromAddr)
	require.Equal(*ans.To, toAddr)

	testData2 := []interface{}{"0x58df1e9cb0572fea48e8ce9d9b787ae557c304657d01890f4fc5ea88a1f44c3e", 1}
	ret, err = svr.web3Server.getTransactionReceipt(testData2)
	require.NoError(err)
	require.Nil(ret)
}

func TestGetTransactionByBlockHashAndIndex(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	header, _ := svr.core.bc.BlockHeaderByHeight(1)
	blkHash := header.HashBlock()

	testData := []interface{}{"0x" + hex.EncodeToString(blkHash[:]), "0x0"}
	ret, err := svr.web3Server.getTransactionByBlockHashAndIndex(testData)
	ans := ret.(transactionObject)
	require.NoError(err)
	require.Equal(ans.Hash, "0x"+hex.EncodeToString(transferHash1[:]))
	fromAddr, _ := ioAddrToEthAddr(identityset.Address(27).String())
	toAddr, _ := ioAddrToEthAddr(identityset.Address(30).String())
	require.Equal(ans.From, fromAddr)
	require.Equal(*ans.To, toAddr)
	require.Equal(ans.Gas, uint64ToHex(20000))
	require.Equal(ans.GasPrice, uint64ToHex(0))

	testData2 := []interface{}{"0x" + hex.EncodeToString(blkHash[:]), "0x10"}
	_, err = svr.web3Server.getTransactionByBlockHashAndIndex(testData2)
	require.Error(err)

	testData3 := []interface{}{"0xa2e8e0c9cafbe93f2b7f7c9d32534bc6fde95f2185e5f2aaa6bf7ebdf1a6610a", "0x0"}
	ret, err = svr.web3Server.getTransactionByBlockHashAndIndex(testData3)
	require.NoError(err)
	require.Nil(ret)
}

func TestGetTransactionByBlockNumberAndIndex(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{"0x1", "0x0"}
	ret, err := svr.web3Server.getTransactionByBlockNumberAndIndex(testData)
	ans := ret.(transactionObject)
	require.NoError(err)
	require.Equal(ans.Hash, "0x"+hex.EncodeToString(transferHash1[:]))
	fromAddr, _ := ioAddrToEthAddr(identityset.Address(27).String())
	toAddr, _ := ioAddrToEthAddr(identityset.Address(30).String())
	require.Equal(ans.From, fromAddr)
	require.Equal(*ans.To, toAddr)
	require.Equal(ans.Gas, uint64ToHex(20000))
	require.Equal(ans.GasPrice, uint64ToHex(0))

	testData2 := []interface{}{"0x1", "0x10"}
	_, err = svr.web3Server.getTransactionByBlockNumberAndIndex(testData2)
	require.Error(err)

	testData3 := []interface{}{"0x10", "0x0"}
	ret, err = svr.web3Server.getTransactionByBlockNumberAndIndex(testData3)
	require.NoError(err)
	require.Nil(ret)
}

func TestNewfilter(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := &filterObject{
		FromBlock: "0x1",
	}
	ret, err := svr.web3Server.newFilter(testData)
	require.NoError(err)
	require.Equal(ret, "0xe10f7dd489b75a36de8e246eb974827fe86a02ed19d9b475a1600cf4f935feff")
}

func TestNewBlockFilter(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	ret, err := svr.web3Server.newBlockFilter()
	require.NoError(err)
	require.Equal("0x71371f8dbaefc4c96d2534163a1b461951c88520cd32bc03b5bfdfe7340bc187", ret)
}

func TestGetFilterChanges(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	filterReq := &filterObject{
		FromBlock: "0x1",
	}
	filterID1, _ := svr.web3Server.newFilter(filterReq)
	ret, err := svr.web3Server.getFilterChanges([]interface{}{filterID1})
	require.NoError(err)
	require.Equal(len(ret.([]logsObject)), 4)
	// request again after last rolling
	ret, err = svr.web3Server.getFilterChanges([]interface{}{filterID1})
	require.NoError(err)
	require.Equal(len(ret.([]logsObject)), 0)

	filterID2, _ := svr.web3Server.newBlockFilter()
	ret2, err := svr.web3Server.getFilterChanges([]interface{}{filterID2})
	require.NoError(err)
	require.Equal(1, len(ret2.([]string)))
	ret3, err := svr.web3Server.getFilterChanges([]interface{}{filterID2})
	require.NoError(err)
	require.Equal(0, len(ret3.([]string)))

}

func TestGetFilterLogs(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	filterReq := &filterObject{
		FromBlock: "0x1",
	}
	filterID, _ := svr.web3Server.newFilter(filterReq)
	testData := []interface{}{filterID}
	ret, err := svr.web3Server.getFilterLogs(testData)
	require.NoError(err)
	require.Equal(len(ret.([]logsObject)), 4)
}

func TestAPICache(t *testing.T) {
	require := require.New(t)

	testKey, testData := strconv.Itoa(rand.Int()), []byte(strconv.Itoa(rand.Int()))
	// local cache
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

func TestGetStorageAt(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	// deploy a contract
	contractCode := "608060405234801561001057600080fd5b50610150806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c14610057575b600080fd5b6100556004803603810190610050919061009d565b610075565b005b61005f61007f565b60405161006c91906100d9565b60405180910390f35b8060008190555050565b60008054905090565b60008135905061009781610103565b92915050565b6000602082840312156100b3576100b26100fe565b5b60006100c184828501610088565b91505092915050565b6100d3816100f4565b82525050565b60006020820190506100ee60008301846100ca565b92915050565b6000819050919050565b600080fd5b61010c816100f4565b811461011757600080fd5b5056fea2646970667358221220c86a8c4dd175f55f5732b75b721d714ceb38a835b87c6cf37cf28c790813e19064736f6c63430008070033"
	contract, _ := deployContractV2(svr, identityset.PrivateKey(13), 1, svr.core.bc.TipHeight(), contractCode)

	contractAddr, _ := ioAddrToEthAddr(contract)
	testData := []interface{}{contractAddr, "0x0"}
	ret, err := svr.web3Server.getStorageAt(testData)
	require.NoError(err)
	// the value of any contract at pos0 is be "0x0000000000000000000000000000000000000000000000000000000000000000"
	require.Equal("0x0000000000000000000000000000000000000000000000000000000000000000", ret)

	failData := [][]interface{}{
		{1},
		{"TEST", "TEST"},
	}
	for _, v := range failData {
		_, err := svr.web3Server.getStorageAt(v)
		require.Error(err)
	}
}

func TestGetNetworkID(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()
	res, _ := svr.web3Server.getNetworkID()
	require.Equal("1", res)
}
