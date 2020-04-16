// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/pkg/version"
)

func TestActionBuilder(t *testing.T) {
	t.Run("default gas price", func(t *testing.T) {
		bd := &EnvelopeBuilder{}
		elp, err := bd.Build()
		assert.NoError(t, err)
		assert.Equal(t, uint32(version.ProtocolVersion), elp.Version())
		assert.Equal(t, uint64(0), elp.Nonce())
		assert.Equal(t, uint64(0), elp.GasLimit())
		assert.Equal(t, big.NewInt(0), elp.GasPrice())
	})
	t.Run("normal case", func(t *testing.T) {
		bd := &EnvelopeBuilder{}
		elp, err := bd.SetVersion(version.ProtocolVersion).
			SetNonce(2).
			SetGasLimit(10003).
			SetGasPrice(big.NewInt(10004)).
			Build()
		assert.NoError(t, err)
		assert.Equal(t, uint32(version.ProtocolVersion), elp.Version())
		assert.Equal(t, uint64(2), elp.Nonce())
		assert.Equal(t, uint64(10003), elp.GasLimit())
		assert.Equal(t, big.NewInt(10004), elp.GasPrice())
	})
	t.Run("invalid gas price", func(t *testing.T) {
		bd := &EnvelopeBuilder{}
		_, err := bd.SetVersion(version.ProtocolVersion).
			SetNonce(2).
			SetGasLimit(10003).
			SetGasPrice(big.NewInt(-10004)).
			Build()
		assert.Error(t, err)
		assert.EqualError(t, ErrGasPrice, err.Error())
	})
}
