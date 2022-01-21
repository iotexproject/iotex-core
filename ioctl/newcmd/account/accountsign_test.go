package account

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/stretchr/testify/require"
)

func TestNewAccountSign(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString", config.English).AnyTimes()

	// test for not invalid account address
	cmd := NewAccountSign(client)
	cmd.Flag("signer").Value.Set("io1rc2d2de7rtuucalsqv4d9ng0h297t63w7wvlph")
	_, err := util.ExecuteCmd(cmd)
	require.Equal("failed to sign message: invalid address #io1rc2d2de7rtuucalsqv4d9ng0h297t63w7wvlph", err.Error())
}
