// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package update

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewUpdateCmd(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("mockTranslationResult",
		config.English).AnyTimes()
	cmd := NewUpdateCmd(client)
	client.EXPECT().Execute(gomock.Any()).Return(nil).Times(1)
	client.EXPECT().ReadSecret().Return("abc", nil).Times(1)
	res, err := util.ExecuteCmd(cmd)
	require.NotNil(t, res)
	require.NoError(t, err)

	expectedError := errors.New("failed to execute bash command")
	client.EXPECT().Execute(gomock.Any()).Return(expectedError).Times(1)
	client.EXPECT().ReadSecret().Return("abc", nil).AnyTimes()
	res, err = util.ExecuteCmd(cmd)
	require.EqualError(t, err, "mockTranslationResult: "+expectedError.Error())
}
