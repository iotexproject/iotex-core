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

	time.Sleep(3 * time.Second)
	err = svr.Stop()
	require.NoError(err)
}

func TestServerV2Error(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	config.SetEVMNetworkID(1)
	svr, bfIndexFile, _ := createServerV2(cfg, false)
	defer func() {
		testutil.CleanupPath(t, bfIndexFile)
	}()

	err := svr.GrpcServer.Start()
	if err != nil {
		svr.GrpcServer.Stop()
	} else {
		err = svr.Start()
		require.Error(err)
		require.Contains(err.Error(), "grpc server failed to listen")
		svr.GrpcServer.Stop()
	}

	svr.core.chainListener = nil
	err = svr.Start()
	require.Error(err)
	require.Contains(err.Error(), "failed to add chainListener")

	svr.tracer = &tracesdk.TracerProvider{}
	err = svr.Stop()
	require.Error(err)
	require.Contains(err.Error(), "failed to shutdown api tracer")
}
