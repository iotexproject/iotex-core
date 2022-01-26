// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewAccountCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationString",
		config.English).AnyTimes()

	oldCryptoSm2 := CryptoSm2
	CryptoSm2 = false
	cmd := NewAccountCreate(client)
	result, err := util.ExecuteCmd(cmd)
	require.NotNil(t, result)
	require.NoError(t, err)

	CryptoSm2 = true
	cmd = NewAccountCreate(client)
	result, err = util.ExecuteCmd(cmd)
	require.NotNil(t, result)
	require.NoError(t, err)
	CryptoSm2 = oldCryptoSm2
}
