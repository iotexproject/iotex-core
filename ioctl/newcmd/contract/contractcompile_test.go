// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contract

import (
	"os/exec"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/ioctl/config"
	"github.com/iotexproject/iotex-core/v2/ioctl/util"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_ioctlclient"
)

func TestNewContractCompileCmd(t *testing.T) {
	skipWithoutSolc(t)
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	client := mock_ioctlclient.NewMockClient(ctrl)

	client.EXPECT().SelectTranslation(gomock.Any()).Return("contract", config.English).AnyTimes()
	client.EXPECT().SetEndpointWithFlag(gomock.Any()).AnyTimes()
	client.EXPECT().SetInsecureWithFlag(gomock.Any()).AnyTimes()

	t.Run("compile contract", func(t *testing.T) {
		cmd := NewContractCompileCmd(client)
		result, err := util.ExecuteCmd(cmd, "A", "../../../action/protocol/execution/testdata-london/array-return.sol")
		require.NoError(err)
		require.Contains(result, "array-return.sol:A")
	})

	t.Run("failed to get source file(s)", func(t *testing.T) {
		expectedErr := errors.New("failed to get source file(s)")
		cmd := NewContractCompileCmd(client)
		_, err := util.ExecuteCmd(cmd, "HelloWorld")
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to find out contract", func(t *testing.T) {
		expectedErr := errors.New("failed to find out contract WrongContract")
		cmd := NewContractCompileCmd(client)
		_, err := util.ExecuteCmd(cmd, "WrongContract", "../../../action/protocol/execution/testdata-london/array-return.sol")
		require.Contains(err.Error(), expectedErr.Error())
	})

	t.Run("failed to compile", func(t *testing.T) {
		expectedErr := errors.New("failed to compile")
		cmd := NewContractCompileCmd(client)
		_, err := util.ExecuteCmd(cmd, "", "")
		require.Contains(err.Error(), expectedErr.Error())
	})
}

func skipWithoutSolc(t *testing.T) {
	if _, err := exec.LookPath(_solCompiler); err != nil {
		t.Skip(err)
	}
}
