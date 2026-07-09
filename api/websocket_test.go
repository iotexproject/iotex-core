// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
)

func TestNewWebsocketHandlerNilLimiter(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	core := NewMockCoreService(ctrl)
	web3 := NewWeb3Handler(core, "", _defaultBatchRequestLimit)

	// nil limiter should be replaced by an (effectively unlimited) limiter
	h := NewWebsocketHandler(core, web3, nil)
	r.NotNil(h.limiter)
	r.True(h.limiter.Allow())
}

// newLocalWSConn spins up an httptest server that upgrades to websocket and
// returns both a client conn and the server-side conn (via a channel).
func newLocalWSConn(t *testing.T) (*websocket.Conn, *websocket.Conn, *httptest.Server) {
	t.Helper()
	serverConnCh := make(chan *websocket.Conn, 1)
	up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		c, err := up.Upgrade(w, req, nil)
		require.NoError(t, err)
		serverConnCh <- c
	}))
	u := "ws" + strings.TrimPrefix(srv.URL, "http")
	client, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(t, err)
	server := <-serverConnCh
	return client, server, srv
}

func TestSafeWebsocketConn(t *testing.T) {
	r := require.New(t)
	client, server, srv := newLocalWSConn(t)
	defer srv.Close()
	defer client.Close()

	safe := &safeWebsocketConn{ws: server}

	// SetWriteDeadline should succeed on a live connection.
	r.NoError(safe.SetWriteDeadline(time.Now().Add(time.Second)))

	// WriteJSON is delivered and readable by the client.
	r.NoError(safe.WriteJSON(map[string]string{"hello": "world"}))
	var got map[string]string
	r.NoError(client.ReadJSON(&got))
	r.Equal("world", got["hello"])

	// WriteMessage is delivered as-is.
	r.NoError(safe.WriteMessage(websocket.TextMessage, []byte("raw")))
	mt, data, err := client.ReadMessage()
	r.NoError(err)
	r.Equal(websocket.TextMessage, mt)
	r.Equal("raw", string(data))

	// concurrent writers must not race or corrupt frames; every write must
	// succeed and every frame must be read back intact.
	const nConcurrent = 20
	writeErrs := make(chan error, nConcurrent)
	var wg sync.WaitGroup
	for i := 0; i < nConcurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			writeErrs <- safe.WriteMessage(websocket.TextMessage, []byte("x"))
		}()
	}
	// drain concurrently so writes don't block on a full buffer, validating
	// each frame as it arrives.
	readErrs := make(chan error, nConcurrent)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for i := 0; i < nConcurrent; i++ {
			mt, data, err := client.ReadMessage()
			if err == nil && (mt != websocket.TextMessage || string(data) != "x") {
				err = errors.Errorf("corrupt frame: type=%d data=%q", mt, data)
			}
			readErrs <- err
		}
	}()
	wg.Wait()
	close(writeErrs)
	for err := range writeErrs {
		r.NoError(err)
	}
	<-readDone
	close(readErrs)
	for err := range readErrs {
		r.NoError(err)
	}

	r.NoError(safe.Close())
}

func TestWebsocketHandlerServeHTTP(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	core := NewMockCoreService(ctrl)
	// Track is deferred on every web3 request; allow it unconditionally.
	core.EXPECT().Track(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	core.EXPECT().EVMNetworkID().Return(uint32(4689)).AnyTimes()

	web3 := NewWeb3Handler(core, "", _defaultBatchRequestLimit)
	wsHandler := NewWebsocketHandler(core, web3, nil)

	srv := httptest.NewServer(wsHandler)
	defer srv.Close()

	u := "ws" + strings.TrimPrefix(srv.URL, "http")
	c, _, err := websocket.DefaultDialer.Dial(u, nil)
	r.NoError(err)
	defer c.Close()

	// eth_chainId maps to coreService.EVMNetworkID() and returns hex(4689)=0x1251
	req := `{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}`
	r.NoError(c.WriteMessage(websocket.TextMessage, []byte(req)))

	r.NoError(c.SetReadDeadline(time.Now().Add(5 * time.Second)))
	_, data, err := c.ReadMessage()
	r.NoError(err)

	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  string `json:"result"`
	}
	r.NoError(json.Unmarshal(data, &resp))
	r.Equal("2.0", resp.JSONRPC)
	r.Equal(1, resp.ID)
	r.Equal("0x1251", resp.Result)
}

func TestWebsocketHandlerServeHTTPParseError(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	core := NewMockCoreService(ctrl)
	core.EXPECT().Track(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	web3 := NewWeb3Handler(core, "", _defaultBatchRequestLimit)
	wsHandler := NewWebsocketHandler(core, web3, nil)

	srv := httptest.NewServer(wsHandler)
	defer srv.Close()

	u := "ws" + strings.TrimPrefix(srv.URL, "http")
	c, _, err := websocket.DefaultDialer.Dial(u, nil)
	r.NoError(err)
	defer c.Close()

	// malformed JSON triggers a parse-error web3 response written back over the socket
	r.NoError(c.WriteMessage(websocket.TextMessage, []byte("not-json")))
	r.NoError(c.SetReadDeadline(time.Now().Add(5 * time.Second)))
	_, data, err := c.ReadMessage()
	r.NoError(err)

	var resp map[string]json.RawMessage
	r.NoError(json.Unmarshal(data, &resp))
	// an error object is present in the response
	_, ok := resp["error"]
	r.True(ok)
}

// ensure the apitypes ResponseWriter used by the handler compiles against the
// expected constructor signature (guards against silent API drift).
var _ = apitypes.NewResponseWriter
