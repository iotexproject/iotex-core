package api

import (
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/mock/mock_apicoreservice"
	"github.com/iotexproject/iotex-core/testutil"
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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	svr := NewWeb3Server(core, testutil.RandomPort(), "", 10)

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
	err := json.Unmarshal(bodyBytes5, &web3Reqs)
	require.NoError(err)
	require.Equal(len(web3Reqs), 2)

	// multiple web3 req2
	request6, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`[{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}]`))
	response6 := getServerResp(svr, request6)
	bodyBytes6, _ := io.ReadAll(response6.Body)
	err = json.Unmarshal(bodyBytes6, &web3Reqs)
	require.NoError(err)
	require.Equal(len(web3Reqs), 1)

	// web3 req without params
	request7, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{"jsonrpc":"2.0","method":"web3_clientVersion","id":67}`))
	// core.EXPECT().ServerMeta().Return("mock str1", "mock str2", "mock str3", "mock str4", "mock str5")
	response7 := getServerResp(svr, request7)
	bodyBytes7, _ := io.ReadAll(response7.Body)
	require.Contains(string(bodyBytes7), "result")
}

func getServerResp(svr *Web3Server, req *http.Request) *httptest.ResponseRecorder {
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	svr.ServeHTTP(resp, req)
	return resp
}

func TestGasPrice(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3svr := NewWeb3Server(core, testutil.RandomPort(), "", 10)
	core.EXPECT().SuggestGasPrice().Return(uint64(1), nil)
	ret, err := web3svr.gasPrice()
	require.NoError(err)
	require.Equal("0x1", ret.(string))

	core.EXPECT().SuggestGasPrice().Return(uint64(0), errors.New("mock gas price error"))
	_, err = web3svr.gasPrice()
	require.Equal("mock gas price error", err.Error())
}

func TestGetChainID(t *testing.T) {

}

func TestGetBlockNumber(t *testing.T) {

}

func TestGetBlockByNumber(t *testing.T) {

}

func TestGetBalance(t *testing.T) {

}

func TestGetTransactionCount(t *testing.T) {

}

func TestCall(t *testing.T) {

}

func TestEstimateGas(t *testing.T) {

}

func TestSendRawTransaction(t *testing.T) {

}

func TestGetCode(t *testing.T) {

}

func TestGetNodeInfo(t *testing.T) {

}

func TestGetBlockTransactionCountByHash(t *testing.T) {

}

func TestGetBlockByHash(t *testing.T) {

}

func TestGetTransactionByHash(t *testing.T) {

}

func TestGetLogs(t *testing.T) {

}

func TestGetTransactionReceipt(t *testing.T) {

}

func TestGetBlockTransactionCountByNumber(t *testing.T) {

}

func TestGetTransactionByBlockHashAndIndex(t *testing.T) {

}

func TestGetTransactionByBlockNumberAndIndex(t *testing.T) {

}

func TestNewfilter(t *testing.T) {

}

func TestNewBlockFilter(t *testing.T) {

}

func TestGetFilterChanges(t *testing.T) {

}

func TestGetFilterLogs(t *testing.T) {

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

}

func TestGetNetworkID(t *testing.T) {

}

func TestEthAccounts(t *testing.T) {

}
