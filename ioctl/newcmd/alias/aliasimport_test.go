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
	cfg := config.Config{
		Aliases: map[string]string{
			"a": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
			"b": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
			"c": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1",
			"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
		},
	}
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslation", config.English).Times(4)
	client.EXPECT().AliasMap().Return(map[string]string{
		"a": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
		"b": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
		"c": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542s1",
		"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx": "io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx",
	}).AnyTimes()
	client.EXPECT().Config().Return(cfg).AnyTimes()

	cmd := NewAliasImportCmd(client)

	result, err := util.ExecuteCmd(cmd, `{"name":"test","address":"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx"}`)

	require.NoError(t, err)
	require.NotNil(t, result)

	result, err = util.ExecuteCmd(cmd, "-F", `{"name":"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx","address":"io1uwnr55vqmhf3xeg5phgurlyl702af6eju542sx"}`)
	require.NoError(t, err)
	require.NotNil(t, result)

	result, err = util.ExecuteCmd(cmd, `""{"name":"d","address":""}""`)
	require.Error(t, err)
	require.NotNil(t, result)

	result, err = util.ExecuteCmd(cmd, "-f", "yaml", `aliases:
- name: mhs2
  address: io19sdfxkwegeaenvxk2kjqf98al52gm56wa2eqks
- name: yqr
  address: io1cl6rl2ev5dfa988qmgzg2x4hfazmp9vn2g66ng
- name: mhs
  address: io1tyc2yt68qx7hmxl2rhssm99jxnhrccz9sneh08`)
	require.NoError(t, err)
	require.NotNil(t, result)

	result, err = util.ExecuteCmd(cmd, "-f", "yaml", `aliases:
name: mhs2
  address: io19sdfxkwegeaenvxk2kjqf98al52gm56wa2eqks`)
	require.Error(t, err)
	require.NotNil(t, result)

	result, err = util.ExecuteCmd(cmd, "-f", "test", "test")
	require.Error(t, err)
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
