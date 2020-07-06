package alias

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewAliasImportCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslation", config.English).Times(4)

	cmd := NewAliasImportCmd(client)

	result, err := util.ExecuteCmd(cmd, `{"name":"a","address":"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx"}`)

	require.NoError(t, err)
	require.NotNil(t, result)

}

// test for list message display
func TestAliasImportMessage_String(t *testing.T) {
	require := require.New(t)
	message := importMessage{}
	message.Imported = append(message.Unimported, alias{Name: "a", Address: "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx"})
	message.Unimported = append(message.Imported, alias{Name: "b", Address: "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx"})
	message.Unimported = append(message.Imported, alias{Name: "c", Address: "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1"})
	str := "0/0 aliases imported\nExisted aliases: a c"
	require.Equal(message.String(), str)
}
