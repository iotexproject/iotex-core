// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignVerify(t *testing.T) {
	pub, pri, err := NewKeyPair()
	assert.Nil(t, err)

	message := []byte("hello iotex message")
	sig := Sign(pri, message)
	assert.True(t, Verify(pub, message, sig))

	wrongMessage := []byte("wrong message")
	assert.False(t, Verify(pub, wrongMessage, sig))
}
