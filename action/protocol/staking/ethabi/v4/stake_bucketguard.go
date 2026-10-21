// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package v4

import (
	"encoding/hex"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/abiutil"
	stakingComm "github.com/iotexproject/iotex-core/v2/action/protocol/staking/ethabi/common"
)

// bucketGuard is a focused getter for the quantum-safe guard record of a
// native bucket (IIP draft #86), following the per-feature getter convention
// described in stake_candidatedeactivation.go.
const _bucketGuardInterfaceABI = `[
	{
		"inputs": [
			{"internalType": "uint64", "name": "index", "type": "uint64"}
		],
		"name": "bucketGuard",
		"outputs": [
			{"internalType": "bool", "name": "guarded", "type": "bool"},
			{"internalType": "uint32", "name": "algID", "type": "uint32"},
			{"internalType": "bytes", "name": "publicKey", "type": "bytes"},
			{"internalType": "uint64", "name": "generation", "type": "uint64"},
			{"internalType": "uint64", "name": "nonce", "type": "uint64"},
			{"internalType": "uint64", "name": "boundHeight", "type": "uint64"}
		],
		"stateMutability": "view",
		"type": "function"
	}
]`

var _bucketGuardMethod abi.Method

func init() {
	_bucketGuardMethod = abiutil.MustLoadMethod(_bucketGuardInterfaceABI, "bucketGuard")
}

type bucketGuardStateContext struct {
	*protocol.BaseStateContext
}

func newBucketGuardStateContext(data []byte) (*bucketGuardStateContext, error) {
	paramsMap := map[string]interface{}{}
	if err := _bucketGuardMethod.Inputs.UnpackIntoMap(paramsMap, data); err != nil {
		return nil, err
	}
	index, ok := paramsMap["index"].(uint64)
	if !ok {
		return nil, stakingComm.ErrDecodeFailure
	}
	methodBytes, err := proto.Marshal(&iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_BUCKET_GUARD,
	})
	if err != nil {
		return nil, err
	}
	argumentsBytes, err := proto.Marshal(&iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_BucketGuard_{
			BucketGuard: &iotexapi.ReadStakingDataRequest_BucketGuard{Index: index},
		},
	})
	if err != nil {
		return nil, err
	}
	return &bucketGuardStateContext{
		&protocol.BaseStateContext{
			Parameter: &protocol.Parameters{
				MethodName: methodBytes,
				Arguments:  [][]byte{argumentsBytes},
			},
			Method: &_bucketGuardMethod,
		},
	}, nil
}

// EncodeToEth packs the BucketGuard record; guarded is true iff a public key
// is installed.
func (r *bucketGuardStateContext) EncodeToEth(resp *iotexapi.ReadStateResponse) (string, error) {
	var g iotextypes.BucketGuard
	if err := proto.Unmarshal(resp.Data, &g); err != nil {
		return "", err
	}
	publicKey := g.GetPublicKey()
	if publicKey == nil {
		publicKey = []byte{}
	}
	data, err := r.Method.Outputs.Pack(len(publicKey) > 0, g.GetAlgID(), publicKey, g.GetGeneration(), g.GetNonce(), g.GetBoundHeight())
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
