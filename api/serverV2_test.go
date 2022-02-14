package api

import (
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
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	err := svr.Start()
	require.NoError(err)

	err = testutil.WaitUntil(100*time.Millisecond, 3*time.Second, func() (bool, error) {
		err = svr.Stop()
		return err == nil, err
	})
	require.NoError(err)

	coreService, ok := svr.core.(*coreService)
	if !ok {
		require.Error(err)
	}
	coreService.chainListener = nil
	err = svr.Start()
	require.Error(err)
	require.Contains(err.Error(), "failed to add chainListener")

	svr.tracer = &tracesdk.TracerProvider{}
	err = svr.Stop()
	require.Error(err)
	require.Contains(err.Error(), "failed to shutdown api tracer")
}
