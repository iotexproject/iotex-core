package api

import (
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

// TODO
func TestServeHTTP(t *testing.T) {

}
