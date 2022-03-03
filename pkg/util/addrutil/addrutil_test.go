// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package addrutil

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestIoAddrToEvmAddr(t *testing.T) {
	t.Run("converts IoTeX address into evm address successfully", func(t *testing.T) {
		ioAddr := identityset.PrivateKey(28)

		ethAddr, err := IoAddrToEvmAddr(ioAddr.PublicKey().Address().String())
		require.NoError(t, err)
		require.Equal(t, ioAddr.PublicKey().Address().Bytes(), ethAddr.Bytes())
	})

	t.Run("failed to convert IoTeX address into evm address", func(t *testing.T) {
		ioAddr := ""
		expectedErr := errors.Errorf("hrp  and address prefix io don't match: invalid bech32 string length %d", len(ioAddr))

		ethAddr, err := IoAddrToEvmAddr(ioAddr)
		require.Error(t, err)
		require.Equal(t, common.Address{}, ethAddr)
		require.Equal(t, err.Error(), expectedErr.Error())
	})
}
