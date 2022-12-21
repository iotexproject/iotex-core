// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/test/mock/mock_apicoreservice"
	mock_apitypes "github.com/iotexproject/iotex-core/test/mock/mock_apiresponder"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestGrpcServer_StreamLogs(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	core := mock_apicoreservice.NewMockCoreService(ctrl)
	grpcSvr := newGRPCHandler(core)

	t.Run("StreamLogsEmptyFilter", func(t *testing.T) {
		err := grpcSvr.StreamLogs(&iotexapi.StreamLogsRequest{}, nil)
		require.Contains(err.Error(), "empty filter")
	})
	t.Run("StreamLogsAddResponderFailed", func(t *testing.T) {
		listener := mock_apitypes.NewMockListener(ctrl)
		listener.EXPECT().AddResponder(gomock.Any()).Return("", errors.New("mock test"))
		core.EXPECT().ChainListener().Return(listener)
		err := grpcSvr.StreamLogs(&iotexapi.StreamLogsRequest{Filter: &iotexapi.LogsFilter{}}, nil)
		require.Contains(err.Error(), "mock test")
	})
	t.Run("StreamLogsSuccess", func(t *testing.T) {
		listener := mock_apitypes.NewMockListener(ctrl)
		listener.EXPECT().AddResponder(gomock.Any()).DoAndReturn(func(g *gRPCLogListener) (string, error) {
			go func() {
				g.errChan <- nil
			}()
			return "", nil
		})
		core.EXPECT().ChainListener().Return(listener)
		err := grpcSvr.StreamLogs(&iotexapi.StreamLogsRequest{Filter: &iotexapi.LogsFilter{}}, nil)
		require.NoError(err)
	})
}
