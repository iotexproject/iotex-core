// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewAccountDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString",
		config.English).AnyTimes()

	testAccountFolder := filepath.Join(os.TempDir(), "testAccount")
	require.NoError(t, os.MkdirAll(testAccountFolder, os.ModePerm))
	defer func() {
		require.NoError(t, os.RemoveAll(testAccountFolder))
	}()
	ks := keystore.NewKeyStore(testAccountFolder,
		keystore.StandardScryptN, keystore.StandardScryptP)
	acc, _ := ks.NewAccount("test")
	accAddr, _ := address.FromBytes(acc.Address.Bytes())

	client.EXPECT().GetAliasMap().DoAndReturn(
		func() map[string]string {
			aliases := make(map[string]string)
			for name, addr := range config.ReadConfig.Aliases {
				aliases[addr] = name
			}
			return aliases
		}).Times(2)

	client.EXPECT().Config().DoAndReturn(
		func() config.Config {
			config.ReadConfig.Wallet = testAccountFolder
			return config.ReadConfig
		}).Times(5)

	CryptoSm2 = false
	client.EXPECT().GetAddress(gomock.Any()).Return(accAddr.String(), nil).Times(2)
	client.EXPECT().NewKeyStore(gomock.Any(), gomock.Any(), gomock.Any()).Return(ks).Times(2)
	client.EXPECT().AskToConfirm(gomock.Any()).Return(false)
	cmd := NewAccountDelete(client)
	_, err := util.ExecuteCmd(cmd)
	require.NoError(t, err)

	client.EXPECT().AskToConfirm(gomock.Any()).Return(true)
	cmd = NewAccountDelete(client)
	_, err = util.ExecuteCmd(cmd)
	require.NoError(t, err)

	CryptoSm2 = true
	priKey2, _ := crypto.GenerateKeySm2()
	addr2 := priKey2.PublicKey().Address()
	pemFilePath := sm2KeyPath(addr2)
	crypto.WritePrivateKeyToPem(pemFilePath, priKey2.(*crypto.P256sm2PrvKey), "test")
	client.EXPECT().GetAddress(gomock.Any()).Return(addr2.String(), nil)
	client.EXPECT().AskToConfirm(gomock.Any()).Return(true)
	cmd = NewAccountDelete(client)
	_, err = util.ExecuteCmd(cmd)
	require.NoError(t, err)
}
