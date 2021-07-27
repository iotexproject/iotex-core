// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewAccountDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString",
		config.English).AnyTimes()

	testAccountFolder := filepath.Join(os.TempDir(), "testAccount")
	_ = os.MkdirAll(testAccountFolder, os.ModePerm)
	ks := keystore.NewKeyStore(testAccountFolder,
		keystore.StandardScryptN, keystore.StandardScryptP)
	acc, _ := ks.NewAccount("test")
	accAddr, _ := address.FromBytes(acc.Address.Bytes())
	client.EXPECT().GetAddress(gomock.Any()).Return(accAddr.String(), nil)
	client.EXPECT().NewKeyStore(gomock.Any(), gomock.Any(), gomock.Any()).Return(ks)

	cmd := NewAccountDelete(client)
	_, err := util.ExecuteCmd(cmd)
	require.NoError(t, err)

	err = os.RemoveAll(testAccountFolder)
	if err != nil {
		log.Fatal(err)
	}
}
