// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

func TestFundSerializeDeserialize(t *testing.T) {
	r := require.New(t)
	f := fund{
		totalBalance:     big.NewInt(1000),
		unclaimedBalance: big.NewInt(400),
	}
	data, err := f.Serialize()
	r.NoError(err)

	var f2 fund
	r.NoError(f2.Deserialize(data))
	r.Equal(0, f2.totalBalance.Cmp(f.totalBalance))
	r.Equal(0, f2.unclaimedBalance.Cmp(f.unclaimedBalance))
}

func TestFundDeserializeError(t *testing.T) {
	r := require.New(t)
	var f fund
	// not a valid protobuf message
	r.Error(f.Deserialize([]byte{0xff, 0xff, 0xff, 0xff}))
}

func TestFundEncodeDecode(t *testing.T) {
	r := require.New(t)
	f := fund{
		totalBalance:     big.NewInt(12345),
		unclaimedBalance: big.NewInt(678),
	}
	v, err := f.Encode()
	r.NoError(err)
	r.NotEmpty(v.PrimaryData)
	r.NotEmpty(v.SecondaryData)

	var f2 fund
	r.NoError(f2.Decode(v))
	r.Equal(0, f2.totalBalance.Cmp(f.totalBalance))
	r.Equal(0, f2.unclaimedBalance.Cmp(f.unclaimedBalance))
}

func TestFundDecodeEmptyDefaultsToZero(t *testing.T) {
	r := require.New(t)
	f := fund{
		totalBalance:     big.NewInt(0),
		unclaimedBalance: big.NewInt(0),
	}
	v, err := f.Encode()
	r.NoError(err)

	var f2 fund
	r.NoError(f2.Decode(v))
	r.Equal(0, f2.totalBalance.Sign())
	r.Equal(0, f2.unclaimedBalance.Sign())
}

func TestIsZero(t *testing.T) {
	r := require.New(t)
	r.True(isZero(nil))
	r.True(isZero(big.NewInt(0)))
	r.False(isZero(big.NewInt(1)))
	r.False(isZero(big.NewInt(-1)))
}

// Depositing a zero amount is a no-op that returns no transaction logs.
func TestDepositZeroAmount(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		logs, err := p.Deposit(ctx, sm, big.NewInt(0), 0)
		require.NoError(t, err)
		require.Nil(t, logs)
	}, nil, false, 0)
}

// DepositGas below the genesis block height (height 0) is bypassed.
func TestDepositGasAtGenesisBypassed(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		gctx := protocol.MustGetBlockCtx(ctx)
		gctx.BlockHeight = 0
		ctx0 := protocol.WithBlockCtx(ctx, gctx)
		logs, err := DepositGas(ctx0, sm, big.NewInt(100))
		require.NoError(t, err)
		require.Nil(t, logs)
	}, nil, false, 0)
}
