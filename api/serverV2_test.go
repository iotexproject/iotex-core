package api

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/mock/mock_apicoreservice"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestServerV2(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	// TODO: mock web3handler
	web3Handler := NewWeb3Handler(core, "")
	svr := &ServerV2{
		core:         core,
		grpcServer:   NewGRPCServer(core, testutil.RandomPort()),
		httpSvr:      NewHTTPServer("", testutil.RandomPort(), newHTTPHandler(web3Handler)),
		websocketSvr: NewHTTPServer("", testutil.RandomPort(), NewWebsocketHandler(web3Handler)),
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
}
