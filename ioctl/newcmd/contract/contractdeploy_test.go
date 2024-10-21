// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement,
// merchantability or fitness for purpose and, to the extent permitted by law,
// all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_ioctlclient"
)

func TestNewContractDeployCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)
	client.EXPECT().SelectTranslation(gomock.Any()).Return("contract", config.English).AnyTimes()

	cmd := NewContractDeployCmd(client)
	result, err := util.ExecuteCmd(cmd)
	require.NoError(err)
	require.Contains(result, "contract\n\n")
}
