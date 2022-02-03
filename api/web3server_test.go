package api

import (
	"encoding/hex"
	"encoding/json"
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

	"github.com/iotexproject/go-pkgs/util"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	_evmNetworkID uint32 = 1
)

func TestGetWeb3Reqs(t *testing.T) {
	require := require.New(t)
	testData := []struct {
		testName  string
		req       *http.Request
		hasHeader bool
		hasError  bool
	}{
		{
			testName:  "EmptyData",
			req:       httptest.NewRequest(http.MethodPost, "http://url.com", strings.NewReader("")),
			hasHeader: true,
			hasError:  true,
		},
		{
			testName:  "InvalidHttpMethod",
			req:       httptest.NewRequest(http.MethodPut, "http://url.com", strings.NewReader("")),
			hasHeader: false,
			hasError:  true,
		},
		{
			testName:  "MissingParamsField",
			req:       httptest.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"web3_clientVersion","id":67}`)),
			hasHeader: true,
			hasError:  false,
		},
		{
			testName:  "Valid",
			req:       httptest.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"web3_clientVersion","params":[],"id":67}`)),
			hasHeader: true,
			hasError:  false,
		},
	}

	for _, test := range testData {
		t.Run(test.testName, func(t *testing.T) {
			if test.hasHeader {
				test.req.Header.Set("Content-Type", contentType)
			}
			_, err := parseWeb3Reqs(test.req)
			if test.hasError {
				require.Error(err)
			} else {
				require.NoError(err)
			}
		})
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
	bodyBytes2, _ := io.ReadAll(response2.Body)
	require.Contains(string(bodyBytes2), "invalid format")

	// missing web3 method
	request3, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"web3_foo","params":[],"id":67}`))
	response3 := getServerResp(svr, request3)
	bodyBytes3, _ := io.ReadAll(response3.Body)
	require.Contains(string(bodyBytes3), "method not found")

	// single web3 req
	request4, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":67}`))
	response4 := getServerResp(svr, request4)
	bodyBytes4, _ := io.ReadAll(response4.Body)
	require.Contains(string(bodyBytes4), "result")

	// multiple web3 req
	request5, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`[{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}, {"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":2}]`))
	response5 := getServerResp(svr, request5)
	bodyBytes5, _ := io.ReadAll(response5.Body)
	var web3Reqs []web3Resp
	_ = json.Unmarshal(bodyBytes5, &web3Reqs)
	require.Equal(len(web3Reqs), 2)

	// web3 req without params
	request6, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"web3_clientVersion","id":67}`))
	response6 := getServerResp(svr, request6)
	bodyBytes6, _ := io.ReadAll(response6.Body)
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
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	ret, _ := svr.web3Server.gasPrice()
	require.Equal(uint64ToHex(1000000000000), ret)
}

func TestGetChainID(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	ret, _ := svr.web3Server.getChainID()
	require.Equal(uint64ToHex(1), ret)
}

func TestGetBlockNumber(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	ret, _ := svr.web3Server.getBlockNumber()
	require.Equal(uint64ToHex(4), ret)
}

func TestGetBlockByNumber(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := []struct {
		data     []interface{}
		expected int
	}{
		{[]interface{}{"1", true}, 1},
		{[]interface{}{"1", false}, 2},
		{[]interface{}{"10", false}, 0},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			ret, err := svr.web3Server.getBlockByNumber(v.data)
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

func TestGetBalance(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := []interface{}{"0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", 1}
	ret, _ := svr.web3Server.getBalance(testData)
	ans, _ := big.NewInt(0).SetString("9999999999999999999999999991", 10)
	require.Equal("0x"+fmt.Sprintf("%x", ans), ret)
}

func TestGetTransactionCount(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := []struct {
		data     []interface{}
		expected int
	}{
		{[]interface{}{"0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", "0x1"}, 2},
		{[]interface{}{"0xDa7e12Ef57c236a06117c5e0d04a228e7181CF36", "pending"}, 2},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			ret, _ := svr.web3Server.getTransactionCount(v.data)
			require.Equal(uint64ToHex(uint64(v.expected)), ret)
		})
	}
}

func TestCall(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := []struct {
		data []interface{}
	}{
		{
			[]interface{}{
				map[string]interface{}{
					"from":     "",
					"to":       "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5",
					"gas":      "0x4e20",
					"gasPrice": "0xe8d4a51000",
					"value":    "0x1",
					"data":     "0x1"},
				1},
		},
		{
			[]interface{}{
				map[string]interface{}{
					"from":     "",
					"to":       "0xb1f8e55c7f64d203c1400b9d8555d050f94adf39",
					"gas":      "0x4e20",
					"gasPrice": "0xe8d4a51000",
					"value":    "0x1",
					"data":     "0x1"},
				1},
		},
	}

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			ret, err := svr.web3Server.call(v.data)
			require.NoError(err)
			fmt.Println(ret)
		})
	}
}

func TestEstimateGas(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

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

	for i, v := range testData {
		t.Run(fmt.Sprintf("%d-%d", i, len(testData)-1), func(t *testing.T) {
			ret, err := svr.web3Server.estimateGas(v.input)
			require.NoError(err)
			require.Equal(ret, uint64ToHex(v.result))
		})
	}
}

func TestSendRawTransaction(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := []interface{}{"f8600180830186a09412745fec82b585f239c01090882eb40702c32b04808025a0b0e1aab5b64d744ae01fc9f1c3e9919844a799e90c23129d611f7efe6aec8a29a0195e28d22d9b280e00d501ff63525bb76f5c87b8646c89d5d9c5485edcb1b498"}
	res, _ := svr.web3Server.sendRawTransaction(testData)
	require.Equal("0x778fd5a054e74e9055bf68ef5f9d559fa306e8ba7dee608d0a3624cca0b63b3e", res)
}

func TestGetCode(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

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
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	_, err := svr.web3Server.getNodeInfo()
	require.NoError(err)
}

func TestGetBlockTransactionCountByHash(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	header, _ := svr.core.bc.BlockHeaderByHeight(1)
	blkHash := header.HashBlock()
	testData := []interface{}{"0x" + hex.EncodeToString(blkHash[:]), 1}
	ret, err := svr.web3Server.getBlockTransactionCountByHash(testData)
	require.NoError(err)
	require.Equal(uint64ToHex(2), ret)
}

func TestGetBlockByHash(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	header, _ := svr.core.bc.BlockHeaderByHeight(1)
	blkHash := header.HashBlock()

	testData := []interface{}{"0x" + hex.EncodeToString(blkHash[:]), false}
	ret, err := svr.web3Server.getBlockByHash(testData)
	require.NoError(err)
	ans := ret.(blockObject)
	require.Equal("0x"+hex.EncodeToString(blkHash[:]), ans.Hash)
	require.Equal(2, len(ans.Transactions))

	testData2 := []interface{}{"0xa2e8e0c9cafbe93f2b7f7c9d32534bc6fde95f2185e5f2aaa6bf7ebdf1a6610a", false}
	ret, err = svr.web3Server.getBlockByHash(testData2)
	require.NoError(err)
	require.Nil(ret)
}

func TestGetTransactionByHash(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := []interface{}{"0x" + hex.EncodeToString(transferHash1[:]), false}
	ret, err := svr.web3Server.getTransactionByHash(testData)
	require.NoError(err)
	require.Equal("0x"+hex.EncodeToString(transferHash1[:]), ret.(transactionObject).Hash)

	testData2 := []interface{}{"0x58df1e9cb0572fea48e8ce9d9b787ae557c304657d01890f4fc5ea88a1f44c3e", false}
	ret, err = svr.web3Server.getTransactionByHash(testData2)
	require.NoError(err)
	require.Nil(ret)
}

func TestGetLogs(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
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

func TestGetTransactionReceipt(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

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

func TestGetBlockTransactionCountByNumber(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	testData := []interface{}{uint64ToHex(1), 1}
	ret, err := svr.web3Server.getBlockTransactionCountByNumber(testData)
	require.NoError(err)
	require.Equal(ret, uint64ToHex(2))
}

func TestGetTransactionByBlockHashAndIndex(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

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
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

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
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	testData := &filterObject{FromBlock: "0x1"}
	ret, err := svr.web3Server.newFilter(testData)
	require.NoError(err)
	require.Equal(ret, "0xe10f7dd489b75a36de8e246eb974827fe86a02ed19d9b475a1600cf4f935feff")
}

func TestNewBlockFilter(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	ret, err := svr.web3Server.newBlockFilter()
	require.NoError(err)
	require.Equal("0x71371f8dbaefc4c96d2534163a1b461951c88520cd32bc03b5bfdfe7340bc187", ret)
}

func TestGetFilterChanges(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	// filter
	filterReq := &filterObject{FromBlock: "0x1"}
	filterID1, _ := svr.web3Server.newFilter(filterReq)
	ret, err := svr.web3Server.getFilterChanges([]interface{}{filterID1})
	require.NoError(err)
	require.Equal(len(ret.([]logsObject)), 4)
	// request again after last rolling
	ret, err = svr.web3Server.getFilterChanges([]interface{}{filterID1})
	require.NoError(err)
	require.Equal(len(ret.([]logsObject)), 0)

	// blockfilter
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
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	filterReq := &filterObject{FromBlock: "0x1"}
	filterID, _ := svr.web3Server.newFilter(filterReq)
	testData := []interface{}{filterID}
	ret, err := svr.web3Server.getFilterLogs(testData)
	require.NoError(err)
	require.Equal(len(ret.([]logsObject)), 4)
}

func TestLocalAPICache(t *testing.T) {
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

func TestGetStorageAt(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

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
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	res, _ := svr.web3Server.getNetworkID()
	require.Equal(fmt.Sprintf("%d", _evmNetworkID), res)
}

func setupTestServer(t *testing.T) (*ServerV2, func()) {
	cfg := newConfig(t)
	config.SetEVMNetworkID(_evmNetworkID)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	return svr,
		func() {
			testutil.CleanupPath(t, bfIndexFile)
		}
}

func TestTraceCall(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	// deploy a contract
	contractCode := "6080604052600a60005534801561001557600080fd5b5061016b806100256000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c1461006b575b600080fd5b610055600480360381019061005091906100b8565b610089565b60405161006291906100f4565b60405180910390f35b61007361009a565b60405161008091906100f4565b60405180910390f35b600081600081905550819050919050565b60008054905090565b6000813590506100b28161011e565b92915050565b6000602082840312156100ce576100cd610119565b5b60006100dc848285016100a3565b91505092915050565b6100ee8161010f565b82525050565b600060208201905061010960008301846100e5565b92915050565b6000819050919050565b600080fd5b6101278161010f565b811461013257600080fd5b5056fea2646970667358221220fdfcbe84666ed5ade4e3d34008002b05251dfe80a32244196a6a37e0f54f882e64736f6c63430008070033"
	contract, _ := deployContractV2(svr, identityset.PrivateKey(13), 1, svr.core.bc.TipHeight(), contractCode)

	contractAddr, _ := ioAddrToEthAddr(contract)
	input := []interface{}{
		map[string]interface{}{
			"from":     "",
			"to":       contractAddr,
			"gas":      "0x0",
			"gasPrice": "0x0",
			"value":    "0x0",
			"data":     "0x6d4ce63c"},
		1}
	res, err := svr.web3Server.traceCall(input)
	require.NoError(err)
	require.Equal("0x000000000000000000000000000000000000000000000000000000000000000a", res.(traceResult).Output)
}

func TestTraceTransaction(t *testing.T) {
	require := require.New(t)
	svr, cleanCallback := setupTestServer(t)
	defer cleanCallback()

	// deploy a contract
	contractCode := "6080604052600a60005534801561001557600080fd5b5061016b806100256000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c806360fe47b11461003b5780636d4ce63c1461006b575b600080fd5b610055600480360381019061005091906100b8565b610089565b60405161006291906100f4565b60405180910390f35b61007361009a565b60405161008091906100f4565b60405180910390f35b600081600081905550819050919050565b60008054905090565b6000813590506100b28161011e565b92915050565b6000602082840312156100ce576100cd610119565b5b60006100dc848285016100a3565b91505092915050565b6100ee8161010f565b82525050565b600060208201905061010960008301846100e5565b92915050565b6000819050919050565b600080fd5b6101278161010f565b811461013257600080fd5b5056fea2646970667358221220fdfcbe84666ed5ade4e3d34008002b05251dfe80a32244196a6a37e0f54f882e64736f6c63430008070033"
	_, _ = deployContractV2(svr, identityset.PrivateKey(13), 1, svr.core.bc.TipHeight(), contractCode)

	input := []interface{}{"f8600180830186a09412745fec82b585f239c01090882eb40702c32b04808025a0b0e1aab5b64d744ae01fc9f1c3e9919844a799e90c23129d611f7efe6aec8a29a0195e28d22d9b280e00d501ff63525bb76f5c87b8646c89d5d9c5485edcb1b498",
		[]interface{}{"trace"}}
	res, err := svr.web3Server.traceRawTransaction(input)
	require.NoError(err)
	require.Equal("0x2710", res.(traceResult).Trace[0].Result.GasUsed)
}
