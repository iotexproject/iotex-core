// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"testing"

	"github.com/facebookgo/clock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestClockOption(t *testing.T) {
	r := require.New(t)
	bc := &blockchain{}
	mockClock := clock.NewMock()
	r.NoError(ClockOption(mockClock)(bc))
	r.Equal(mockClock, bc.clk)
}

func TestBlockValidatorOption(t *testing.T) {
	r := require.New(t)
	bc := &blockchain{}
	r.Nil(bc.blockValidator)
	// a non-nil validator gets wired in
	v := block.NewValidator(nil)
	r.NoError(BlockValidatorOption(v)(bc))
	r.NotNil(bc.blockValidator)
}

func TestSkipSidecarValidationOption(t *testing.T) {
	r := require.New(t)
	cfg := BlockValidationCfg{}
	r.False(cfg.skipSidecarValidation)
	SkipSidecarValidationOption()(&cfg)
	r.True(cfg.skipSidecarValidation)
}

func TestWithProducerPrivateKey(t *testing.T) {
	r := require.New(t)
	opts := MintOptions{}
	pk := identityset.PrivateKey(0)
	WithProducerPrivateKey(pk)(&opts)
	r.Equal(pk, opts.ProducerPrivateKey)
}
