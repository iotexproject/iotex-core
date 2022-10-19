// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/util"
	"github.com/iotexproject/iotex-core/test/mock/mock_ioctlclient"
)

func TestNewContractCmd(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("contract", config.English).AnyTimes()
	client.EXPECT().SetEndpointWithFlag(gomock.Any()).Do(func(_ func(*string, string, string, string)) {})
	client.EXPECT().SetInsecureWithFlag(gomock.Any()).Do(func(_ func(*bool, string, bool, string)) {})

	cmd := NewContractCmd(client)
	result, err := util.ExecuteCmd(cmd)
	require.NoError(err)
	require.Contains(result, "Available Commands")
}
