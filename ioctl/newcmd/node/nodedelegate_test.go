package node

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewNodeDelegateCmd(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return(
		"mockTranslationString", config.English).AnyTimes()

	apiServiceClient := mock_apiserviceclient.NewMockServiceClient(ctrl)

	client.EXPECT().APIServiceClient(gomock.Any()).Return(
		apiServiceClient, nil).AnyTimes()

	chainMetaResponse := &iotexapi.GetChainMetaResponse{ChainMeta: &iotextypes.ChainMeta{}}
	apiServiceClient.EXPECT().GetChainMeta(gomock.Any(), gomock.Any()).Return(chainMetaResponse, nil).AnyTimes()

	epochMetaResponse := &iotexapi.GetEpochMetaResponse{EpochData: &iotextypes.EpochData{}}
	apiServiceClient.EXPECT().GetEpochMeta(gomock.Any(), gomock.Any()).Return(epochMetaResponse, nil).AnyTimes()

	kickoutList := &iotexapi.ReadStateResponse{}
	apiServiceClient.EXPECT().ReadState(gomock.Any(), gomock.Any()).Return(kickoutList, nil).AnyTimes()

	cmd := NewNodeDelegateCmd(client)

	result, err := util.ExecuteCmd(cmd)
	require.NotNil(t, result)
	require.NoError(t, err)

	//Todo add unit test for nextDelegates situation

	cmd.Flags().BoolVarP(&nextEpoch, "next-epoch", "n", true, "query delegate of upcoming epoch")
	var epochNum uint64
	epochNum = 7644
	apiServiceClient.EXPECT().ReadState(
		gomock.Any(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte("poll"),
			MethodName: []byte("ActiveBlockProducersByEpoch"),
			Arguments:  [][]byte{byteutil.Uint64ToBytes(epochNum)},
		}).Return(&iotexapi.ReadStateResponse{
		Data: []byte("0")},
		nil).AnyTimes()

	apiServiceClient.EXPECT().ReadState(gomock.Any(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("poll"),
		MethodName: []byte("BlockProducersByEpoch"),
		Arguments:  [][]byte{byteutil.Uint64ToBytes(epochNum)},
	}).Return(&iotexapi.ReadStateResponse{
		Data: []byte("0")},
		nil).AnyTimes()
	result, err = util.ExecuteCmd(cmd)
	require.NotNil(t, result)
	require.NoError(t, err)

}
