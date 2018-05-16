// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package beacon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBeacon(t *testing.T) {
	b := NewBeacon("asdf")

	expected := "asdf"
	assert.Equal(t, expected, b.GetSeed())

	b.NextEpoch()
	expected = "f0e4c2f76c58916ec258f246851bea091d14d4247a2fc3e18694461b1816e13b"
	assert.Equal(t, expected, b.GetSeed())

	b.NextEpoch()
	expected = "7624e1f89ce009f8ec7e6e39781a42c0a27fa38f94db4f05f78b0f301007e06a"
	assert.Equal(t, expected, b.GetSeed())

	b.NextEpoch()
	expected = "a2053032e8177afe4e024021b5b002dfdffcc0d73754a0feebe8196778784b61"
	assert.Equal(t, expected, b.GetSeed())
}
