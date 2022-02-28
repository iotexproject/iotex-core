package api

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestServerV2Start(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, _, _, _, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()
	ctx := context.Background()

	err := svr.Start(ctx)
	require.NoError(err)

	err = testutil.WaitUntil(100*time.Millisecond, 3*time.Second, func() (bool, error) {
		err = svr.Stop(ctx)
		return err == nil, err
	})
	require.NoError(err)

	coreService, ok := svr.core.(*coreService)
	require.True(ok)
	// TODO: move to unit test in coreservice
	coreService.chainListener = nil
	err = svr.Start(ctx)
	require.Error(err)
	require.Contains(err.Error(), "failed to add chainListener")

	svr.tracer = &tracesdk.TracerProvider{}
	err = svr.Stop(ctx)
	require.Error(err)
	require.Contains(err.Error(), "failed to shutdown api tracer")
}
