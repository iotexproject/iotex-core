package alias

import (
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/stretchr/testify/require"
	"testing"
)

// test for alias list command
func TestNewAliasListCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).Times(2)
	cfg := config.Config{
		Aliases: map[string]string{
			"a": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
			"b": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
			"c": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1",
		},
	}
	client.EXPECT().Config().Return(cfg).AnyTimes()
	cmd := NewAliasListCmd(client)
	res, err := util.ExecuteCmd(cmd)
	require.NotNil(t, res)
	require.NoError(t, err)
}

// test for list message display
func TestAliasListMessage_String(t *testing.T) {
	require := require.New(t)
	message := aliasListMessage{AliasNumber: 3}
	message.AliasList = append(message.AliasList, alias{Name: "a", Address: "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx"})
	message.AliasList = append(message.AliasList, alias{Name: "b", Address: "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx"})
	message.AliasList = append(message.AliasList, alias{Name: "c", Address: "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1"})
	str := "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx - a\nio1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx - b\nio1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1 - c"
	require.Equal(message.String(), str)
}
