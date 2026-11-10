// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package v4

import (
	"encoding/hex"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestBucketGuardStateContext(t *testing.T) {
	r := require.New(t)
	data, err := _bucketGuardMethod.Inputs.Pack(uint64(42))
	r.NoError(err)
	sc, err := BuildReadStateRequest(append(append([]byte{}, _bucketGuardMethod.ID...), data...))
	r.NoError(err)

	var m iotexapi.ReadStakingDataMethod
	r.NoError(proto.Unmarshal(sc.Parameters().MethodName, &m))
	r.Equal(iotexapi.ReadStakingDataMethod_BUCKET_GUARD, m.GetMethod())
	var req iotexapi.ReadStakingDataRequest
	r.NoError(proto.Unmarshal(sc.Parameters().Arguments[0], &req))
	r.EqualValues(42, req.GetBucketGuard().GetIndex())

	decode := func(g *iotextypes.BucketGuard) []interface{} {
		raw, err := proto.Marshal(g)
		r.NoError(err)
		out, err := sc.EncodeToEth(&iotexapi.ReadStateResponse{Data: raw})
		r.NoError(err)
		b, err := hex.DecodeString(out)
		r.NoError(err)
		vals, err := _bucketGuardMethod.Outputs.Unpack(b)
		r.NoError(err)
		return vals
	}
	pk := []byte{1, 2, 3}
	r.Equal([]interface{}{true, uint32(0x12), pk, uint64(2), uint64(5), uint64(100)},
		decode(&iotextypes.BucketGuard{AlgID: 0x12, PublicKey: pk, Generation: 2, Nonce: 5, BoundHeight: 100}))
	r.Equal([]interface{}{false, uint32(0), []byte{}, uint64(3), uint64(9), uint64(0)},
		decode(&iotextypes.BucketGuard{Generation: 3, Nonce: 9}), "tombstone")
	r.Equal([]interface{}{false, uint32(0), []byte{}, uint64(0), uint64(0), uint64(0)},
		decode(&iotextypes.BucketGuard{}), "no record")
}
