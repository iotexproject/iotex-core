// Copyright (c) 2026 IoTeX Foundation
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestEstimateGasBucketGuardPreflight(t *testing.T) {
	for _, failure := range []error{nil, bucketguard.ErrGuardSignature, bucketguard.ErrGuardScope, bucketguard.ErrVaultUnbinding, bucketguard.ErrVaultClosed, bucketguard.ErrCancelCooldown} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			core := NewMockCoreService(gomock.NewController(t))
			core.EXPECT().ChainID().Return(uint32(1))
			core.EXPECT().EVMNetworkID().Return(uint32(1))
			a, err := action.NewVaultAction(&bucketguard.VaultAction{Method: "vaultInitiateExit", Index: 7})
			require.NoError(t, err)
			gas, err := a.IntrinsicGas()
			require.NoError(t, err)
			core.EXPECT().EstimateBucketGuardGas(gomock.Any(), gomock.Any(), identityset.Address(3)).DoAndReturn(func(_ context.Context, elp action.Envelope, _ address.Address) (uint64, error) {
				require.IsType(t, a, elp.Action())
				require.EqualValues(t, 9, elp.Nonce())
				return gas, failure
			})
			in := gjson.Parse(fmt.Sprintf(`{"params":[{"from":%q,"to":%q,"nonce":"0x9","data":"0x%s"}]}`, identityset.Address(3).Hex(), mustEthTo(t, a), hex.EncodeToString(mustEthData(t, a))))
			handler := &web3Handler{coreService: core}
			result, err := handler.estimateGas(context.Background(), &in)
			if failure != nil {
				require.True(t, errors.Is(err, failure))
			} else {
				require.NoError(t, err)
				require.Equal(t, uint64ToHex(gas), result)
			}
		})
	}
}
func mustEthTo(t *testing.T, a *action.VaultAction) string {
	t.Helper()
	to, err := a.EthTo()
	require.NoError(t, err)
	return to.Hex()
}
func mustEthData(t *testing.T, a *action.VaultAction) []byte {
	t.Helper()
	data, err := a.EthData()
	require.NoError(t, err)
	return data
}
