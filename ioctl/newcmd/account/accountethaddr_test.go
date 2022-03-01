// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewAccountEthAddr(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("", config.English).AnyTimes()
	client.EXPECT().PrintInfo(gomock.Any()).Times(2)

	t.Run("when an iotex address was given", func(t *testing.T) {
		client.EXPECT().Address(gomock.Any()).Return(identityset.Address(28).String(), nil)
		cmd := NewAccountEthAddr(client)
		_, err := util.ExecuteCmd(cmd, "io187evpmjdankjh0g5dfz83w2z3p23ljhn4s9jw7")
		require.NoError(err)
	})

	t.Run("when an ethereum address was given", func(t *testing.T) {
		client.EXPECT().Address(gomock.Any()).Return(identityset.Address(28).String(), nil)
		cmd := NewAccountEthAddr(client)
		_, err := util.ExecuteCmd(cmd, "0x7c13866F9253DEf79e20034eDD011e1d69E67fe5")
		require.NoError(err)
	})

	t.Run("cannot find address for alias", func(t *testing.T) {
		expectedErr := errors.New("cannot find address for alias ")
		client.EXPECT().Address(gomock.Any()).Return("", expectedErr)
		cmd := NewAccountEthAddr(client)
		_, err := util.ExecuteCmd(cmd, "")
		require.Contains(err.Error(), expectedErr.Error())
	})
}
