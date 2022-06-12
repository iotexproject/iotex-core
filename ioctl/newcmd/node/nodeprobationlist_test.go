package node

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

func Test_NewNodeProbationListCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return(
		"mockTranslationString", config.English).AnyTimes()
	client.EXPECT().Config().Return(config.ReadConfig).AnyTimes()
	client.EXPECT().AliasMap().Return(map[string]string{
		"a": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
		"b": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
		"c": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1",
		"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
	}).AnyTimes()

	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(
		apiServiceClient, nil).AnyTimes()
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(
		&iotexapi.GetChainMetaResponse{ChainMeta: &iotextypes.ChainMeta{Epoch: &iotextypes.EpochData{Num: 4}}}, nil,
	)

	pbInfo := make(map[string]uint32, 1)
	pbInfo["test"] = 2
	pbInfo["test2"] = 4
	pb := &vote.ProbationList{ProbationInfo: pbInfo, IntensityRate: 100}
	serializedData, err := pb.Serialize()
	require.NoError(err)

	probationList := &iotexapi.ReadStateResponse{Data: serializedData}
	apiServiceClient.EXPECT().ReadState(gomock.Any(), gomock.Any()).Return(probationList, nil).AnyTimes()

	cmd := NewNodeProbationListCmd(client)
	result, err := util.ExecuteCmd(cmd)
	require.NotNil(t, result)
	require.NoError(err)
}

func Test_NewNodeProbationListCmd_ROLLDPOSNotRegistered(t *testing.T) {
	// previous test sets the epoch num, so we have to reset to check the error
	_epochNum = 0
	require := require.New(t)
	ctrl := gomock.NewController(t)

	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return(
		"mockTranslationString", config.English).AnyTimes()
	client.EXPECT().Config().Return(config.ReadConfig).AnyTimes()
	client.EXPECT().AliasMap().Return(map[string]string{
		"a": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
	}).AnyTimes()

	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(
		apiServiceClient, nil).AnyTimes()
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(
		&iotexapi.GetChainMetaResponse{ChainMeta: &iotextypes.ChainMeta{}}, nil,
	)

	cmd := NewNodeProbationListCmd(client)
	result, err := util.ExecuteCmd(cmd)
	require.NotNil(t, result)
	require.Contains(err.Error(), "ROLLDPOS is not registered")
}

func Test_NewNodeProbationListCmd_ChainMetaError(t *testing.T) {
	// previous test sets the epoch num, so we have to reset to check the error
	_epochNum = 0
	require := require.New(t)
	ctrl := gomock.NewController(t)

	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return(
		"mockTranslationString", config.English).AnyTimes()
	client.EXPECT().Config().Return(config.ReadConfig).AnyTimes()
	client.EXPECT().AliasMap().Return(map[string]string{
		"a": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
	}).AnyTimes()

	apiServiceClient := mock_iotexapi.NewMockAPIServiceClient(ctrl)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(
		apiServiceClient, nil).AnyTimes()
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(
		nil, errors.New("problem fetching chain meta"),
	)

	cmd := NewNodeProbationListCmd(client)
	result, err := util.ExecuteCmd(cmd)
	require.NotNil(t, result)
	require.Contains(err.Error(), "failed to get chain meta")
}
