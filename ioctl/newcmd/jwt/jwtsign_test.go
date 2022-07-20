package jwt

import (
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/newcmd/action"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewJwtSignCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("jwt", config.English).AnyTimes()
	client.EXPECT().IsCryptoSm2().Return(false).Times(2)

	ks := keystore.NewKeyStore(t.TempDir(), 2, 1)
	acc, err := ks.NewAccount("")
	require.NoError(err)
	accAddr, err := address.FromBytes(acc.Address.Bytes())
	require.NoError(err)

	client.EXPECT().ReadSecret().Return("", nil)
	client.EXPECT().NewKeyStore().Return(ks).Times(2)
	client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr.String(), nil)
	client.EXPECT().Address(gomock.Any()).Return(accAddr.String(), nil).AnyTimes()

	cmd := NewJwtSignCmd(client)
	action.RegisterWriteCommand(client, cmd)
	result, err := util.ExecuteCmd(cmd)
	require.NoError(err)
	require.Contains(result, "address: "+accAddr.String())
}
