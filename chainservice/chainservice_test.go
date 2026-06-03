// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
)

func TestChainService_Filter_NilHeader(t *testing.T) {
	r := require.New(t)
	cs := &ChainService{}

	cases := []struct {
		name string
		msg  *iotextypes.Block
	}{
		{"nil block", nil},
		{"nil header", &iotextypes.Block{Header: nil}},
		{"nil header core", &iotextypes.Block{Header: &iotextypes.BlockHeader{Core: nil}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r.NotPanics(func() {
				r.False(cs.Filter(iotexrpc.MessageType_BLOCK, tc.msg, 10))
			})
		})
	}

	// Non-BLOCK messages always pass without inspecting payload.
	r.True(cs.Filter(iotexrpc.MessageType_ACTION, nil, 10))
}

func TestChainService_ReportFullness_NilHeader(t *testing.T) {
	r := require.New(t)
	cs := &ChainService{}

	cases := []struct {
		name string
		msg  *iotextypes.Block
	}{
		{"nil block", nil},
		{"nil header", &iotextypes.Block{Header: nil}},
		{"nil header core", &iotextypes.Block{Header: &iotextypes.BlockHeader{Core: nil}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r.NotPanics(func() {
				cs.ReportFullness(nil, iotexrpc.MessageType_BLOCK, tc.msg, 0.5)
			})
		})
	}
}
