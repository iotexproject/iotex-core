package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	. "github.com/agiledragon/gomonkey/v2"
	"github.com/golang/mock/gomock"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"golang.org/x/time/rate"

	"github.com/iotexproject/iotex-core/test/mock/mock_apicoreservice"
)

func TestServerV2(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		core = mock_apicoreservice.NewMockCoreService(ctrl)
		svr  = &ServerV2{
			core:         core,
			grpcServer:   &GRPCServer{},
			httpSvr:      &HTTPServer{},
			websocketSvr: &HTTPServer{},
		}
		ctx = context.Background()
	)

	t.Run("start-stop succeed", func(t *testing.T) {
		t.Run("startSucceed", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			core.EXPECT().Start(gomock.Any()).Return(nil).Times(1)

			p = p.ApplyMethodReturn(&GRPCServer{}, "Start", nil)
			outputs := []OutputCell{
				{Values: Params{nil}},
				{Values: Params{nil}},
			}
			p = p.ApplyMethodSeq(&HTTPServer{}, "Start", outputs)

			err := svr.Start(ctx)
			require.NoError(err)
		})

		t.Run("stopSucceed", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			outputs := []OutputCell{
				{Values: Params{nil}},
				{Values: Params{nil}},
			}
			p = p.ApplyMethodSeq(&HTTPServer{}, "Stop", outputs)
			p = p.ApplyMethodReturn(&GRPCServer{}, "Stop", nil)

			core.EXPECT().Stop(gomock.Any()).Return(nil).Times(1)
			err := svr.Stop(ctx)
			require.NoError(err)
		})
	})

	t.Run("start failed", func(t *testing.T) {
		t.Run("FailedToStartCore", func(t *testing.T) {
			expectErr := errors.New("failed to add chainListener")
			core.EXPECT().Start(gomock.Any()).Return(expectErr).Times(1)
			err := svr.Start(ctx)
			require.Contains(err.Error(), expectErr.Error())
		})

		t.Run("FailedToStartGrpc", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			core.EXPECT().Start(gomock.Any()).Return(nil).Times(1)

			p = p.ApplyMethodReturn(&GRPCServer{}, "Start", errors.New(t.Name()))

			err := svr.Start(ctx)
			require.ErrorContains(err, t.Name())
		})

		t.Run("FailedToStartHttp", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			core.EXPECT().Start(gomock.Any()).Return(nil).Times(1)

			p = p.ApplyMethodReturn(&GRPCServer{}, "Start", nil)
			p = p.ApplyMethodReturn(&HTTPServer{}, "Start", errors.New(t.Name()))

			err := svr.Start(ctx)
			require.ErrorContains(err, t.Name())
		})

		t.Run("FailedToStartWebSocket", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			core.EXPECT().Start(gomock.Any()).Return(nil).Times(1)

			p = p.ApplyMethodReturn(&GRPCServer{}, "Start", nil)
			outputs := []OutputCell{
				{Values: Params{nil}},
				{Values: Params{errors.New(t.Name())}},
			}
			p = p.ApplyMethodSeq(&HTTPServer{}, "Start", outputs)

			err := svr.Start(ctx)
			require.ErrorContains(err, t.Name())
		})
	})

	t.Run("stop failed", func(t *testing.T) {
		t.Run("FailedToStopWebSocket", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			p = p.ApplyMethodReturn(&HTTPServer{}, "Stop", errors.New(t.Name()))

			err := svr.Stop(ctx)
			require.ErrorContains(err, t.Name())
		})

		t.Run("FailedToStopHttp", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			outputs := []OutputCell{
				{Values: Params{nil}},
				{Values: Params{errors.New(t.Name())}},
			}
			p = p.ApplyMethodSeq(&HTTPServer{}, "Stop", outputs)

			err := svr.Stop(ctx)
			require.ErrorContains(err, t.Name())
		})

		t.Run("FailedToStopGrpc", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			outputs := []OutputCell{
				{Values: Params{nil}},
				{Values: Params{nil}},
			}
			p = p.ApplyMethodSeq(&HTTPServer{}, "Stop", outputs)
			p = p.ApplyMethodReturn(&GRPCServer{}, "Stop", errors.New(t.Name()))

			err := svr.Stop(ctx)
			require.ErrorContains(err, t.Name())
		})

		t.Run("FailedToStopCore", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			outputs := []OutputCell{
				{Values: Params{nil}},
				{Values: Params{nil}},
			}
			p = p.ApplyMethodSeq(&HTTPServer{}, "Stop", outputs)
			p = p.ApplyMethodReturn(&GRPCServer{}, "Stop", nil)

			expectErr := errors.New("failed to shutdown api tracer")
			core.EXPECT().Stop(gomock.Any()).Return(expectErr).Times(1)
			err := svr.Stop(ctx)
			require.Contains(err.Error(), expectErr.Error())
		})

		t.Run("FailedToShutdownTrace", func(t *testing.T) {
			p := NewPatches()
			defer p.Reset()

			p = p.ApplyMethodReturn(&tracesdk.TracerProvider{}, "Shutdown", errors.New(t.Name()))

			svr.tracer = &tracesdk.TracerProvider{}
			err := svr.Stop(ctx)
			require.ErrorContains(err, t.Name())
		})
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
		require.Greater(10, i)
	})
}
