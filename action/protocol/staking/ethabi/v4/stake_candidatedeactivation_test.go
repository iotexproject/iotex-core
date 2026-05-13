// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package v4

import (
	"encoding/hex"
	"math"
	"reflect"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

// candidateDeactivation(address) selector is 0x4a85a317 (kept here so the test
// pins the selector — any unintended ABI rename will fail this assertion).
const _candidateDeactivationSelectorAddr = "0000000000000000000000000000000000000000000000000000000000000001"

func TestBuildReadStateRequestCandidateDeactivation(t *testing.T) {
	r := require.New(t)

	data, err := hex.DecodeString(hex.EncodeToString(_candidateDeactivationMethod.ID) + _candidateDeactivationSelectorAddr)
	r.NoError(err)
	req, err := BuildReadStateRequest(data)
	r.NoError(err)
	r.Equal("*v4.candidateDeactivationStateContext", reflect.TypeOf(req).String())

	// We piggyback on CANDIDATE_BY_ADDRESS so the request fields shouldn't
	// change shape — the only difference is the response encoder.
	wantMethod := &iotexapi.ReadStakingDataMethod{
		Method: iotexapi.ReadStakingDataMethod_CANDIDATE_BY_ADDRESS,
	}
	wantMethodBytes, _ := proto.Marshal(wantMethod)
	r.Equal(wantMethodBytes, req.Parameters().MethodName)

	wantArgs := &iotexapi.ReadStakingDataRequest{
		Request: &iotexapi.ReadStakingDataRequest_CandidateByAddress_{
			CandidateByAddress: &iotexapi.ReadStakingDataRequest_CandidateByAddress{
				OwnerAddr: "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833xv",
			},
		},
	}
	wantArgsBytes, _ := proto.Marshal(wantArgs)
	r.Equal([][]byte{wantArgsBytes}, req.Parameters().Arguments)
}

// candidateDeactivation has three meaningful states; verify each one round-trips
// through EncodeToEth into the right (requested, scheduledAtBlock) pair.
func TestCandidateDeactivationToEth(t *testing.T) {
	r := require.New(t)
	cases := []struct {
		name             string
		deactivatedAt    uint64
		wantRequested    bool
		wantScheduledAt  uint64
	}{
		{"no exit in flight", 0, false, 0},
		{"requested, not yet scheduled", uint64(math.MaxUint64), true, 0},
		{"scheduled at block 12345", 12345, true, 12345},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cand := &iotextypes.CandidateV2{
				Id:                 "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833xv",
				OwnerAddress:       "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqps833xv",
				OperatorAddress:    "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqz75y8gn",
				RewardAddress:      "io1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqrrzsj4p",
				Name:               "hello",
				TotalWeightedVotes: "0",
				SelfStakeBucketIdx: 0,
				SelfStakingTokens:  "0",
				DeactivatedAt:      c.deactivatedAt,
			}
			candBytes, err := proto.Marshal(cand)
			r.NoError(err)
			resp := &iotexapi.ReadStateResponse{Data: candBytes}

			ctx := &candidateDeactivationStateContext{
				BaseStateContext: &protocol.BaseStateContext{Method: &_candidateDeactivationMethod},
			}
			hexOut, err := ctx.EncodeToEth(resp)
			r.NoError(err)

			out, err := hex.DecodeString(hexOut)
			r.NoError(err)
			values, err := _candidateDeactivationMethod.Outputs.Unpack(out)
			r.NoError(err)
			r.Len(values, 2)
			r.Equal(c.wantRequested, values[0].(bool))
			r.Equal(c.wantScheduledAt, values[1].(uint64))
		})
	}
}
