// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package beacon

import (
	"testing"

	"encoding/hex"
	"github.com/stretchr/testify/assert"
)

func decodeHash(in string) []byte {
	hash, _ := hex.DecodeString(in)
	return hash
}

func TestBeacon(t *testing.T) {
	b, err := NewBeacon()
	assert.Nil(t, err)

	assert.Equal(t, decodeHash("39646536333036623038313538633432333333306637613237323433613161356362653339626664373634663037383138343337383832643231323431353637"), b.GetSeed())

	b.NextEpoch()
	assert.Equal(t, decodeHash("19ef9c860e73e6dc17169dab7f331938b162834a94a034253c7958dca71dfcba9789148576b7b734398e82f43666bbc5eb41bebf7fdab305016d18aa98c25681"), b.GetSeed())

	b.NextEpoch()
	assert.Equal(t, decodeHash("1a2c5d909c40cd705e0464c0e804a2d1ae60477c17a3651b385a6ec2fe659296e1efdec3e4531b3b27433a2a137abd2eeb60a260aff1028cb7a71402a35af2b5"), b.GetSeed())
}
