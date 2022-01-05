// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/test/mock/mock_apiserviceclient"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

func TestNewAccountNonce(t *testing.T) {
	passwds := []string{
		"test",
		func() string {
			nonce := strconv.FormatInt(rand.Int63(), 10)
			return "3dj,<>@@SF{}rj0ZF#" + nonce
		}(),
	}

	for _, passwd := range passwds {
		execNewAccount(t, passwd)
	}
}

func execNewAccount(t *testing.T, passwd string) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()

	testAccountFolder := filepath.Join(os.TempDir(), "testAccount")
	require.NoError(t, os.MkdirAll(testAccountFolder, os.ModePerm))
	defer func() {
		require.NoError(t, os.RemoveAll(testAccountFolder))
	}()
	ks := keystore.NewKeyStore(testAccountFolder,
		keystore.StandardScryptN, keystore.StandardScryptP)

	acc, err := ks.NewAccount(passwd)
	require.NoError(t, err)

	accAddr, err := address.FromBytes(acc.Address.Bytes())
	require.NoError(t, err)
	client.EXPECT().GetAddress(gomock.Any()).Return(accAddr.String(), nil)

	apiServiceClient := mock_apiserviceclient.NewMockServiceClient(ctrl)
	client.EXPECT().APIServiceClient(gomock.Any()).Return(apiServiceClient, nil).Times(1)

	accountResponse := &iotexapi.GetAccountResponse{AccountMeta: &iotextypes.AccountMeta{}}
	apiServiceClient.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(accountResponse, nil).Times(1)

	cmd := NewAccountNonce(client)
	result, err := util.ExecuteCmd(cmd)
	require.NotNil(t, result)
	require.NoError(t, err)
}
