// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package distributedlog

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
)

func TestUnpackRoundTrip(t *testing.T) {
	r := require.New(t)
	in := happyArgs()
	topics, data, err := Pack(in)
	r.NoError(err)

	out, err := Unpack(topics, data)
	r.NoError(err)
	r.Equal(in.Epoch, out.Epoch)
	r.Equal(in.Delegate.String(), out.Delegate.String())
	r.Zero(in.VoterAmount.Cmp(out.VoterAmount))
	r.Len(out.Voters, len(in.Voters))
	for i := range in.Voters {
		r.Equal(in.Voters[i].String(), out.Voters[i].String())
		r.Equal(in.Recipients[i].String(), out.Recipients[i].String())
		r.Zero(in.Amounts[i].Cmp(out.Amounts[i]))
	}
	r.Equal(in.CompoundBucketIDs, out.CompoundBucketIDs)
	r.Equal(in.Compounded, out.Compounded)
}

func TestUnpackEmptyVoterList(t *testing.T) {
	r := require.New(t)
	in := happyArgs()
	in.VoterAmount = new(big.Int)
	in.Voters = nil
	in.Recipients = nil
	in.Amounts = nil
	in.CompoundBucketIDs = nil
	in.Compounded = nil
	topics, data, err := Pack(in)
	r.NoError(err)

	out, err := Unpack(topics, data)
	r.NoError(err)
	r.Empty(out.Voters)
	r.Empty(out.Recipients)
	r.Empty(out.Amounts)
	r.Empty(out.CompoundBucketIDs)
	r.Empty(out.Compounded)
}

func TestExportedABISurface(t *testing.T) {
	r := require.New(t)
	parsed, err := ABI()
	r.NoError(err)
	ev, ok := parsed.Events[EventName]
	r.True(ok)
	r.Equal(EventSignature, ev.Sig)

	topic0, err := Topic0()
	r.NoError(err)
	r.Equal(hash.Hash256(ev.ID), topic0)

	// Mutating an ABI returned to a consumer must not corrupt the encoder cache.
	delete(parsed.Events, EventName)
	_, _, err = Pack(happyArgs())
	r.NoError(err)
}

func TestUnpackClassifiesForeignAndMalformedLogs(t *testing.T) {
	topics, data, err := Pack(happyArgs())
	require.NoError(t, err)

	t.Run("no topics", func(t *testing.T) {
		_, err := Unpack(nil, data)
		require.ErrorIs(t, err, ErrNotDelegateVoterRewardsDistributed)
	})
	t.Run("foreign selector", func(t *testing.T) {
		foreign := append(action.Topics(nil), topics...)
		foreign[0] = hash.Hash256b([]byte("other event"))
		_, err := Unpack(foreign, data)
		require.ErrorIs(t, err, ErrNotDelegateVoterRewardsDistributed)
	})
	t.Run("retired selector", func(t *testing.T) {
		retired := append(action.Topics(nil), topics...)
		retired[0] = hash.Hash256b([]byte(
			"DelegateDistributed(uint64,address,uint256,address[],address[],uint256[],uint64[],bool[])",
		))
		_, err := Unpack(retired, data)
		require.ErrorIs(t, err, ErrNotDelegateVoterRewardsDistributed)
	})
	t.Run("matching selector wrong topic count", func(t *testing.T) {
		_, err := Unpack(topics[:1], data)
		require.ErrorIs(t, err, ErrMalformedLog)
	})
	t.Run("truncated data", func(t *testing.T) {
		_, err := Unpack(topics, data[:len(data)/2])
		require.ErrorIs(t, err, ErrMalformedLog)
	})
}

func TestUnpackRejectsInvalidIndexedPadding(t *testing.T) {
	topics, data, err := Pack(happyArgs())
	require.NoError(t, err)

	t.Run("epoch", func(t *testing.T) {
		malformed := append(action.Topics(nil), topics...)
		malformed[1][0] = 1
		_, err := Unpack(malformed, data)
		require.ErrorIs(t, err, ErrMalformedLog)
	})
	t.Run("delegate", func(t *testing.T) {
		malformed := append(action.Topics(nil), topics...)
		malformed[2][0] = 1
		_, err := Unpack(malformed, data)
		require.ErrorIs(t, err, ErrMalformedLog)
	})
}

func TestUnpackRejectsRaggedArrays(t *testing.T) {
	r := require.New(t)
	args := happyArgs()
	topics, _, err := Pack(args)
	r.NoError(err)
	parsed, err := ABI()
	r.NoError(err)
	data, err := parsed.Events[EventName].Inputs.NonIndexed().Pack(
		big.NewInt(3),
		[]common.Address{common.BytesToAddress(args.Voters[0].Bytes())},
		[]common.Address{},
		[]*big.Int{big.NewInt(3)},
		[]uint64{0},
		[]bool{false},
	)
	r.NoError(err)

	_, err = Unpack(topics, data)
	r.ErrorIs(err, ErrParallelArrayLengthMismatch)
}
