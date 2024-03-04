package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/websocket"
	"github.com/iotexproject/iotex-core/test/mock/mock_apicoreservice"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/stretchr/testify/require"
)

func TestWebsocket(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3Handler := NewWeb3Handler(core, "", _defaultBatchRequestLimit)
	ws := NewHTTPServer("", testutil.RandomPort(), NewWebsocketHandler(web3Handler))

	ctx := context.Background()

	require.NoError(ws.Start(ctx))
	defer func() {
		require.NoError(ws.Stop(ctx))
	}()
	msg := []byte(`{"jsonrpc":"2.0","method":"eth_subscribe","params":["logs",{"address":"0x1a4d3b66c2f4b6b46e51f6e6e7b3c0b0e9e7e6e7"}],"id":1}`)
	echo := func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			mt, message, err := c.ReadMessage()
			if err != nil {
				break
			}
			err = c.WriteMessage(mt, message)
			if err != nil {
				break
			}
		}
	}
	s := httptest.NewServer(http.HandlerFunc(echo))
	defer s.Close()

	u := "ws" + strings.TrimPrefix(s.URL, "http")
	c, _, err := websocket.DefaultDialer.Dial(u, nil)
	require.NoError(err)
	safeWs := &safeWebsocketConn{ws: c}
	require.NoError(safeWs.SetWriteDeadline(testutil.TimestampNow().Add(time.Hour)))
	require.NoError(safeWs.WriteMessage(websocket.TextMessage, msg))
	require.NoError(safeWs.WriteJSON(msg))
	err = c.WriteMessage(websocket.TextMessage, []byte{0})
	require.NoError(err)
	_, _, err = c.ReadMessage()
	require.NoError(err)
	require.NoError(safeWs.Close())
}
