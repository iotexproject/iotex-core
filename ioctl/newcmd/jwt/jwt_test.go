package jwt

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_ioctlclient"
)

func TestNewJwtCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("jwt", config.English).AnyTimes()

	cmd := NewJwtCmd(client)
	result, err := util.ExecuteCmd(cmd)
	require.NoError(err)
	require.Contains(result, "Available Commands")
}
