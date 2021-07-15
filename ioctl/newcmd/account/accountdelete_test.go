// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewAccountDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString",
		config.English).AnyTimes()

	client.EXPECT().GetAddress(gomock.Any()).Return("io1kuz56knh9g7x3ht36t2pgfd8fz5g6rxrh5a8dn", nil)

	ks := keystore.NewKeyStore(config.ReadConfig.Wallet,
		keystore.StandardScryptN, keystore.StandardScryptP)
	ks.NewAccount("test")
	client.EXPECT().NewKeyStore(gomock.Any(), gomock.Any(), gomock.Any()).Return(ks)
	cmd := NewAccountDelete(client)
	result, err := util.ExecuteCmd(cmd)
	require.NotNil(t, result)
	require.Error(t, err)
}
