package jwt

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewJwtSignCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)

	ks := keystore.NewKeyStore(t.TempDir(), 2, 1)
	acc, err := ks.NewAccount("")
	require.NoError(err)
	accAddr, err := address.FromBytes(acc.Address.Bytes())
	require.NoError(err)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("jwt", config.English).AnyTimes()
	client.EXPECT().IsCryptoSm2().Return(false).Times(4)
	client.EXPECT().ReadSecret().Return("", nil).Times(2)
	client.EXPECT().NewKeyStore().Return(ks).Times(4)
	client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return(accAddr.String(), nil).Times(2)
	client.EXPECT().Address(gomock.Any()).Return(accAddr.String(), nil).AnyTimes()

	t.Run("sign jwt with no arg", func(t *testing.T) {
		cmd := NewJwtSignCmd(client)
		result, err := util.ExecuteCmd(cmd)
		require.NoError(err)
		require.Contains(result, "address: "+accAddr.String())
		require.Contains(result, `"exp": "0"`)
	})

	t.Run("sign jwt with arg", func(t *testing.T) {
		cmd := NewJwtSignCmd(client)
		result, err := util.ExecuteCmd(cmd, "--with-arguments", `{"exp":"-10"}`)
		require.NoError(err)
		require.Contains(result, "address: "+accAddr.String())
		require.Contains(result, `"exp": "-10"`)
	})

	t.Run("failed to get signer address", func(t *testing.T) {
		expectedErr := errors.New("failed to get signer address")
		client.EXPECT().AddressWithDefaultIfNotExist(gomock.Any()).Return("", expectedErr)

		cmd := NewJwtSignCmd(client)
		_, err := util.ExecuteCmd(cmd)
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to unmarshal arguments", func(t *testing.T) {
		expectedErr := errors.New("failed to unmarshal arguments")

		cmd := NewJwtSignCmd(client)
		_, err := util.ExecuteCmd(cmd, "--with-arguments", "test")
		require.Contains(err.Error(), expectedErr.Error())
	})
}
