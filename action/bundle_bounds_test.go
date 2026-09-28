// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// A bundle arrives over an untrusted p2p path and every action in it costs a
// full signature recovery at decode. The count must be bounded before any of
// that work is done.
func TestBundleLoadProtoBoundsActionCount(t *testing.T) {
	r := require.New(t)

	sk := identityset.PrivateKey(27)
	tsf, err := SignedTransfer(identityset.Address(28).String(), sk, 1, big.NewInt(1), nil, 100000, big.NewInt(1000000000000))
	r.NoError(err)

	pbActions := func(n int) []*iotextypes.Action {
		acts := make([]*iotextypes.Action, n)
		for i := range acts {
			acts[i] = tsf.Proto()
		}
		return acts
	}
	d := (&Deserializer{}).SetEvmNetworkID(1)

	oversized := &iotextypes.Bundle{Actions: pbActions(MaxBundleActions + 1), TargetBlockNumber: 10}
	r.ErrorIs(NewBundle().LoadProto(oversized, d), ErrBundleTooLarge)

	atLimit := &iotextypes.Bundle{Actions: pbActions(MaxBundleActions), TargetBlockNumber: 10}
	b := NewBundle()
	r.NoError(b.LoadProto(atLimit, d))
	r.Equal(MaxBundleActions, b.Len())
}
