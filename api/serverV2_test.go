package api

import (
	"context"
	"testing"

	. "github.com/agiledragon/gomonkey/v2"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"

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
}
