package api

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
			req:      httptest.NewRequest(http.MethodPost, "http://url.com", strings.NewReader("{\"jsonrpc\":\"2.0\",\"method\":\"web3_clientVersion\",\"id\":67}")),
			header:   true,
			hasError: true,
		},
		// success
		{
			req:      httptest.NewRequest(http.MethodPost, "http://url.com", strings.NewReader("{\"jsonrpc\":\"2.0\",\"method\":\"web3_clientVersion\",\"params\":[],\"id\":67}")),
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
	svr := &Server{}

	// wrong http method
	request1, _ := http.NewRequest(http.MethodGet, "http://url.com", nil)
	response1 := getServerResp(svr, request1)
	require.Equal(response1.Result().StatusCode, http.StatusMethodNotAllowed)

	// invalid web3 req
	request2, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader("{\"jsonrpc\":\"2.0\",\"method\":\"web3_clientVersion\",\"id\":67}"))
	response2 := getServerResp(svr, request2)
	bodyBytes2, _ := ioutil.ReadAll(response2.Body)
	require.Contains(string(bodyBytes2), "failed to parse")

	// missing web3 method
	request3, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader("{\"jsonrpc\":\"2.0\",\"method\":\"web3_foo\",\"params\":[],\"id\":67}"))
	response3 := getServerResp(svr, request3)
	bodyBytes3, _ := ioutil.ReadAll(response3.Body)
	require.Contains(string(bodyBytes3), "method not found")

	// single web3 req
	request4, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader("{\"jsonrpc\":\"2.0\",\"method\":\"eth_syncing\",\"params\":[],\"id\":67}"))
	response4 := getServerResp(svr, request4)
	bodyBytes4, _ := ioutil.ReadAll(response4.Body)
	require.Contains(string(bodyBytes4), "result")

	// multiple web3 req
	request5, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader("[{\"jsonrpc\":\"2.0\",\"method\":\"eth_syncing\",\"params\":[],\"id\":1}, {\"jsonrpc\":\"2.0\",\"method\":\"eth_syncing\",\"params\":[],\"id\":2}]"))
	response5 := getServerResp(svr, request5)
	bodyBytes5, _ := ioutil.ReadAll(response5.Body)
	var web3Reqs []web3Resp
	_ = json.Unmarshal(bodyBytes5, &web3Reqs)
	require.Equal(len(web3Reqs), 2)
}

func getServerResp(svr *Server, req *http.Request) *httptest.ResponseRecorder {
	req.Header.Set("Content-Type", contentType)
	resp := httptest.NewRecorder()
	svr.ServeHTTP(resp, req)
	return resp
}
