// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewNodeProbationlistCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	chainMetaResponse := &iotexapi.GetChainMetaResponse{ChainMeta: &iotextypes.ChainMeta{
		Epoch: &iotextypes.EpochData{
			Num: 0,
		},
	}}

	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(12)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil).Times(10)

	t.Run("failed to get chain meta", func(t *testing.T) {
		expectedErr := errors.New("failed to get chain meta")
		apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(nil, expectedErr).Times(1)

		cmd := NewNodeProbationlistCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Contains(err.Error(), "failed to get chain meta")
	})

	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).Times(3)

	t.Run("query probation list", func(t *testing.T) {
		probationList := &iotexapi.ReadStateResponse{}

		apiServiceClient.EXPECT().ReadState(gomock.Any(), gomock.Any()).Return(probationList, nil).Times(1)

		cmd := NewNodeProbationlistCmd(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Contains(result, "ProbationList : []")
	})

	t.Run("epochNum > 0", func(t *testing.T) {
		d, _ := vote.NewProbationList(1).Serialize()
		probationList := &iotexapi.ReadStateResponse{
			Data: d,
		}

		apiServiceClient.EXPECT().ReadState(gomock.Any(), gomock.Any()).Return(probationList, nil).Times(1)

		cmd := NewNodeProbationlistCmd(client)
		result, err := util.ExecuteCmd(cmd, "-e", "1")
		require.NoError(err)
		require.Contains(result, "ProbationList : []")
	})

	t.Run("failed to get probation list", func(t *testing.T) {
		apiServiceClient.EXPECT().ReadState(gomock.Any(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte("poll"),
			MethodName: []byte("ProbationListByEpoch"),
			Arguments:  [][]byte{[]byte("0")},
		}).Return(&iotexapi.ReadStateResponse{
			Data: []byte("0")},
			nil)

		cmd := NewNodeProbationlistCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Contains(err.Error(), "failed to get probation list")
	})
}
