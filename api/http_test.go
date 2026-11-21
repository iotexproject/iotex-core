package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_web3server"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestServeHTTP(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	handler := mock_web3server.NewMockWeb3Handler(ctrl)
	handler.EXPECT().HandlePOSTReq(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	svr := newHTTPHandler(handler, 10)

	t.Run("WrongHTTPMethod", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "http://url.com", nil)
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		svr.ServeHTTP(resp, req)
		require.Equal(http.StatusOK, resp.Result().StatusCode)
		bytes, _ := io.ReadAll(resp.Result().Body)
		require.Equal("IoTeX RPC endpoint is ready.", string(bytes))
	})

	t.Run("Success", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "http://url.com", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		svr.ServeHTTP(resp, req)
		require.Equal(http.StatusOK, resp.Result().StatusCode)
	})
}

func TestServeHTTPRequestBodyLimit(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	handler := mock_web3server.NewMockWeb3Handler(ctrl)
	var readErr error
	handler.EXPECT().HandlePOSTReq(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, r io.Reader, _ apitypes.Web3ResponseWriter) error {
			_, readErr = io.Copy(io.Discard, r)
			return readErr
		}).Times(2)
	svr := newHTTPHandler(handler, 10)

	for _, c := range []struct {
		size    int
		tooLong bool
	}{
		{maxMessageSize, false},
		{maxMessageSize + 1, true},
	} {
		req, _ := http.NewRequest(http.MethodPost, "http://url.com", bytes.NewReader(make([]byte, c.size)))
		svr.ServeHTTP(httptest.NewRecorder(), req)
		var maxErr *http.MaxBytesError
		require.Equal(c.tooLong, errors.As(readErr, &maxErr), "body size %d", c.size)
	}
}

func TestServerStartStop(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	handler := mock_web3server.NewMockWeb3Handler(ctrl)
	svr := NewHTTPServer("", testutil.RandomPort(), newHTTPHandler(handler, 10))

	err := svr.Start(context.Background())
	require.NoError(err)
	err = testutil.WaitUntil(100*time.Millisecond, 3*time.Second, func() (bool, error) {
		err = svr.Stop(context.Background())
		return err == nil, err
	})
	require.NoError(err)
}
