// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
)

func TestRegistryCtx(t *testing.T) {
	require := require.New(t)
	_, ok := GetRegistry(context.Background())
	require.False(ok)
	require.Panics(func() { MustGetRegistry(context.Background()) }, "Miss registry context")
	reg := &Registry{}
	ctx := WithRegistry(context.Background(), reg)
	require.NotNil(ctx)
	regFromCtx, ok := GetRegistry(ctx)
	require.True(ok)
	require.Equal(reg, regFromCtx)
	require.Equal(reg, MustGetRegistry(ctx))
}

func TestWithBlockchainCtx(t *testing.T) {
	require := require.New(t)
	bcCtx := BlockchainCtx{}
	require.NotNil(WithBlockchainCtx(context.Background(), bcCtx))
}

func TestGetBlockchainCtx(t *testing.T) {
	require := require.New(t)
	bcCtx := BlockchainCtx{}
	ctx := WithBlockchainCtx(context.Background(), bcCtx)
	require.NotNil(ctx)
	_, ok := GetBlockchainCtx(ctx)
	require.True(ok)
}

func TestMustGetBlockchainCtx(t *testing.T) {
	require := require.New(t)
	bcCtx := BlockchainCtx{}
	ctx := WithBlockchainCtx(context.Background(), bcCtx)
	require.NotNil(ctx)
	// Case I: Normal
	ret := MustGetBlockchainCtx(ctx)
	require.NotNil(ret)
	// Case II: Panic
	require.Panics(func() { MustGetBlockchainCtx(context.Background()) }, "Miss blockchain context")
}

func TestWithBlockCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	blkCtx := BlockCtx{
		BlockHeight:    1111,
		BlockTimeStamp: time.Now(),
		GasLimit:       1,
		Producer:       addr,
	}
	require.NotNil(WithBlockCtx(context.Background(), blkCtx))
}

func TestGetBlockCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	blkCtx := BlockCtx{
		BlockHeight:    1111,
		BlockTimeStamp: time.Now(),
		GasLimit:       1,
		Producer:       addr,
	}
	ctx := WithBlockCtx(context.Background(), blkCtx)
	require.NotNil(ctx)
	ret, ok := GetBlockCtx(ctx)
	require.True(ok)
	require.Equal(uint64(1111), ret.BlockHeight)
}

func TestMustGetBlockCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	blkCtx := BlockCtx{
		BlockHeight:    1111,
		BlockTimeStamp: time.Now(),
		GasLimit:       1,
		Producer:       addr,
	}
	ctx := WithBlockCtx(context.Background(), blkCtx)
	require.NotNil(ctx)
	// Case I: Normal
	ret := MustGetBlockCtx(ctx)
	require.Equal(uint64(1111), ret.BlockHeight)
	// Case II: Panic
	require.Panics(func() { MustGetBlockCtx(context.Background()) }, "Miss block context")
}

func TestWithActionCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	actionCtx := ActionCtx{
		Caller:       addr,
		ActionHash:   hash.ZeroHash256,
		GasPrice:     nil,
		IntrinsicGas: 0,
		Nonce:        0,
	}
	require.NotNil(WithActionCtx(context.Background(), actionCtx))
}

func TestGetActionCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	actionCtx := ActionCtx{
		Caller:       addr,
		ActionHash:   hash.ZeroHash256,
		GasPrice:     nil,
		IntrinsicGas: 0,
		Nonce:        0,
	}
	ctx := WithActionCtx(context.Background(), actionCtx)
	require.NotNil(ctx)
	ret, ok := GetActionCtx(ctx)
	require.True(ok)
	require.Equal(hash.ZeroHash256, ret.ActionHash)
}

func TestMustGetActionCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	actionCtx := ActionCtx{
		Caller:       addr,
		ActionHash:   hash.ZeroHash256,
		GasPrice:     nil,
		IntrinsicGas: 0,
		Nonce:        0,
	}
	ctx := WithActionCtx(context.Background(), actionCtx)
	require.NotNil(ctx)
	// Case I: Normal
	ret := MustGetActionCtx(ctx)
	require.Equal(hash.ZeroHash256, ret.ActionHash)
	// Case II: Panic
	require.Panics(func() { MustGetActionCtx(context.Background()) }, "Miss action context")
}

func TestWithVMConfigCtx(t *testing.T) {
	require := require.New(t)
	require.NotNil(WithVMConfigCtx(context.Background(), vm.Config{Debug: true}))
}

func TestGetVMConfigCtx(t *testing.T) {
	require := require.New(t)
	ctx := WithVMConfigCtx(context.Background(), vm.Config{Debug: true})
	require.NotNil(ctx)
	ret, ok := GetVMConfigCtx(ctx)
	require.True(ok)
	require.True(ret.Debug)
}
