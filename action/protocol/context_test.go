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

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/config"

	"github.com/iotexproject/iotex-address/address"

	"github.com/stretchr/testify/require"
)

func TestWithRunActionsCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	actionCtx := RunActionsCtx{1, time.Now(), 1, config.Default.Genesis, addr, addr, hash.ZeroHash256, nil, 0, 0, false, nil}
	require.NotNil(WithRunActionsCtx(context.Background(), actionCtx))
}

func TestGetRunActionsCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	actionCtx := RunActionsCtx{1111, time.Now(), 1, config.Default.Genesis, addr, addr, hash.ZeroHash256, nil, 0, 0, false, nil}
	ctx := WithRunActionsCtx(context.Background(), actionCtx)
	require.NotNil(ctx)
	ret, ok := GetRunActionsCtx(ctx)
	require.True(ok)
	require.Equal(uint64(1111), ret.BlockHeight)
}

func TestMustGetRunActionsCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	actionCtx := RunActionsCtx{1111, time.Now(), 1, config.Default.Genesis, addr, addr, hash.ZeroHash256, nil, 0, 0, false, nil}
	ctx := WithRunActionsCtx(context.Background(), actionCtx)
	require.NotNil(ctx)
	// Case I: Normal
	ret := MustGetRunActionsCtx(ctx)
	require.Equal(uint64(1111), ret.BlockHeight)
	// Case II: Panic
	require.Panics(func() { MustGetRunActionsCtx(context.Background()) }, "Miss run actions context")
}

func TestWithValidateActionsCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	validateCtx := ValidateActionsCtx{1, "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms", addr, config.Default.Genesis}
	require.NotNil(WithValidateActionsCtx(context.Background(), validateCtx))
}
func TestGetValidateActionsCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	validateCtx := ValidateActionsCtx{1111, "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms", addr, config.Default.Genesis}
	ctx := WithValidateActionsCtx(context.Background(), validateCtx)
	require.NotNil(ctx)
	ret, ok := GetValidateActionsCtx(ctx)
	require.True(ok)
	require.Equal(uint64(1111), ret.BlockHeight)
}
func TestMustGetValidateActionsCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	validateCtx := ValidateActionsCtx{1111, "io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms", addr, config.Default.Genesis}
	ctx := WithValidateActionsCtx(context.Background(), validateCtx)
	require.NotNil(ctx)
	// Case I: Normal
	ret := MustGetValidateActionsCtx(ctx)
	require.Equal(uint64(1111), ret.BlockHeight)
	// Case II: Panic
	require.Panics(func() { MustGetValidateActionsCtx(context.Background()) }, "Miss validate action context")
}

func TestWithBlockchainCtx(t *testing.T) {
	require := require.New(t)
	bcCtx := BlockchainCtx{config.Default.Genesis, false, nil}
	require.NotNil(WithBlockchainCtx(context.Background(), bcCtx))
}

func TestGetBlockchainCtx(t *testing.T) {
	require := require.New(t)
	bcCtx := BlockchainCtx{config.Default.Genesis, false, nil}
	ctx := WithBlockchainCtx(context.Background(), bcCtx)
	require.NotNil(ctx)
	ret, ok := GetBlockchainCtx(ctx)
	require.True(ok)
	require.Equal(false, ret.History)
}

func TestMustGetBlockchainCtx(t *testing.T) {
	require := require.New(t)
	bcCtx := BlockchainCtx{config.Default.Genesis, false, nil}
	ctx := WithBlockchainCtx(context.Background(), bcCtx)
	require.NotNil(ctx)
	// Case I: Normal
	ret := MustGetBlockchainCtx(ctx)
	require.Equal(false, ret.History)
	// Case II: Panic
	require.Panics(func() { MustGetBlockchainCtx(context.Background()) }, "Miss blockchain context")
}

func TestWithBlockCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	blkCtx := BlockCtx{1111, time.Now(), 1, addr}
	require.NotNil(WithBlockCtx(context.Background(), blkCtx))
}

func TestGetBlockCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	blkCtx := BlockCtx{1111, time.Now(), 1, addr}
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
	blkCtx := BlockCtx{1111, time.Now(), 1, addr}
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
	actionCtx := ActionCtx{addr, hash.ZeroHash256, nil, 0, 0}
	require.NotNil(WithActionCtx(context.Background(), actionCtx))
}

func TestGetActionCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	actionCtx := ActionCtx{addr, hash.ZeroHash256, nil, 0, 0}
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
	actionCtx := ActionCtx{addr, hash.ZeroHash256, nil, 0, 0}
	ctx := WithActionCtx(context.Background(), actionCtx)
	require.NotNil(ctx)
	// Case I: Normal
	ret := MustGetActionCtx(ctx)
	require.Equal(hash.ZeroHash256, ret.ActionHash)
	// Case II: Panic
	require.Panics(func() { MustGetActionCtx(context.Background()) }, "Miss action context")
}
