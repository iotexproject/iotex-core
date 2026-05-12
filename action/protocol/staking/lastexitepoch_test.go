// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

func TestLastExitEpoch_SerializeRoundTrip(t *testing.T) {
	r := require.New(t)
	for _, epoch := range []uint64{0, 1, 42, 1 << 20, ^uint64(0)} {
		in := &lastExitEpoch{epoch: epoch}
		data, err := in.Serialize()
		r.NoError(err)
		out := &lastExitEpoch{}
		r.NoError(out.Deserialize(data))
		r.Equal(in.epoch, out.epoch)
	}
}

func TestLastExitEpoch_EncodesSingleValueShape(t *testing.T) {
	r := require.New(t)
	le := &lastExitEpoch{epoch: 12345}
	keys, values, err := le.Encodes()
	r.NoError(err)
	// kvListStorage stores under `prefix + key` for each entry. lastExitEpoch
	// is a single-value state, so it must encode to exactly one entry with
	// an empty suffix key — otherwise the key would shift away from where
	// PutState/GetState expect it.
	r.Len(keys, 1)
	r.Len(values, 1)
	r.Empty(keys[0])
	// PrimaryData carries the protobuf-encoded epoch. SecondaryData /
	// AuxiliaryData stay empty for this state.
	r.NotEmpty(values[0].PrimaryData)
	r.Empty(values[0].SecondaryData)
	r.Empty(values[0].AuxiliaryData)
}

func TestLastExitEpoch_EncodesDecodesRoundTrip(t *testing.T) {
	r := require.New(t)
	for _, epoch := range []uint64{0, 1, 7, 100, 1 << 32, ^uint64(0)} {
		in := &lastExitEpoch{epoch: epoch}
		keys, values, err := in.Encodes()
		r.NoError(err)
		out := &lastExitEpoch{}
		r.NoError(out.Decodes(keys, values))
		r.Equal(in.epoch, out.epoch)
	}
}

func TestLastExitEpoch_DecodesEmptyReturnsError(t *testing.T) {
	r := require.New(t)
	out := &lastExitEpoch{}
	err := out.Decodes(nil, nil)
	r.Error(err)
	err = out.Decodes([][]byte{}, []systemcontracts.GenericValue{})
	r.Error(err)
}

// kvListContainer mirrors the (unexported) interface in
// state/factory/erigonstore.kvListStorage. *lastExitEpoch must satisfy it so
// erigon-backed archive state factories can store/load the state — without
// these methods, handleScheduleCandidateDeactivation panics with
// "object of type *staking.lastExitEpoch does not supported" when archive is
// enabled.
type kvListContainer interface {
	Encodes() ([][]byte, []systemcontracts.GenericValue, error)
	Decodes(keys [][]byte, values []systemcontracts.GenericValue) error
}

func TestLastExitEpoch_ImplementsKVListContainer(t *testing.T) {
	var _ kvListContainer = (*lastExitEpoch)(nil)
}
