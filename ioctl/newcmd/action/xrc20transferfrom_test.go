package action

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi/mock_iotexapi"
)

func TestNewXrc20TransferFrom(t *testing.T) {
	// r := require.New(t)
	ctr := gomock.NewController(t)
	cmd := mock_ioctlclient.NewMockClient(ctr)
	api := mock_iotexapi.NewMockAPIServiceClient(ctr)

	cmd.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).Times(6)
	cmd.EXPECT().APIServiceClient().Return(api, nil).Times(2)
	cmd.EXPECT().Alias(gomock.Any()).Return("producer", nil).Times(3)
}
