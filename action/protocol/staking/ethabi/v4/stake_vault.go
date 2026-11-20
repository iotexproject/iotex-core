// Copyright (c) 2026 IoTeX Foundation
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package v4

import (
	"encoding/hex"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/abiutil"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

const vaultGetterABI = bucketguard.VaultGettersABI

var vaultGetter = abiutil.MustLoadMethod(vaultGetterABI, "vaultBucket")
var stakingGuardGetter = abiutil.MustLoadMethod(vaultGetterABI, "stakingGuardV2")

type vaultStateContext struct{ *protocol.BaseStateContext }

func newVaultStateContext(method *abi.Method, data []byte) (protocol.StateContext, error) {
	if _, err := method.Inputs.Unpack(data); err != nil {
		return nil, err
	}
	return &vaultStateContext{&protocol.BaseStateContext{Parameter: &protocol.Parameters{MethodName: []byte(method.Name), Arguments: [][]byte{data}}, Method: method}}, nil
}
func (c *vaultStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	return hex.EncodeToString(resp.Data), nil
}
