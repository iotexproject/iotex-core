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
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/iotexproject/iotex-core/test/mock/mock_apicoreservice"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestServerV2(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	web3Handler := NewWeb3Handler(core, "", _defaultBatchRequestLimit)
	svr := &ServerV2{
		core:         core,
		grpcServer:   NewGRPCServer(core, nil, testutil.RandomPort()),
		httpSvr:      NewHTTPServer("", testutil.RandomPort(), newHTTPHandler(web3Handler)),
		websocketSvr: NewHTTPServer("", testutil.RandomPort(), NewWebsocketHandler(web3Handler, nil)),
	}
	ctx := context.Background()

	t.Run("start-stop succeed", func(t *testing.T) {
		core.EXPECT().Start(gomock.Any()).Return(nil).Times(1)
		err := svr.Start(ctx)
		require.NoError(err)

		core.EXPECT().Stop(gomock.Any()).Return(nil).Times(1)
		err = testutil.WaitUntil(100*time.Millisecond, 3*time.Second, func() (bool, error) {
			err = svr.Stop(ctx)
			return err == nil, err
		})
		require.NoError(err)
	})

	t.Run("start failed", func(t *testing.T) {
		expectErr := errors.New("failed to add chainListener")
		core.EXPECT().Start(gomock.Any()).Return(expectErr).Times(1)
		err := svr.Start(ctx)
		require.Contains(err.Error(), expectErr.Error())
	})

	t.Run("stop failed", func(t *testing.T) {
		expectErr := errors.New("failed to shutdown api tracer")
		core.EXPECT().Stop(gomock.Any()).Return(expectErr).Times(1)
		err := svr.Stop(ctx)
		require.Contains(err.Error(), expectErr.Error())
	})

	t.Run("websocket rate limit", func(t *testing.T) {
		// set the limiter to 1 request per second
		limiter := rate.NewLimiter(1, 1)
		echo := func(w http.ResponseWriter, r *http.Request) {
			c, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer c.Close()
			for {
				if err := limiter.Wait(ctx); err != nil {
					return
				}
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
		defer c.Close()
		i := 0
		timeout := time.After(3 * time.Second)
	LOOP:
		for {
			select {
			case <-timeout:
				break LOOP
			default:
				err := c.WriteMessage(websocket.TextMessage, []byte{0})
				require.NoError(err)
				_, _, err = c.ReadMessage()
				require.NoError(err)
				i++
			}
		}
		require.Equal(4, i)
	})
}
