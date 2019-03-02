// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package crypto

import (
	"bytes"
	"testing"

	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/stretchr/testify/assert"
)

func TestCryptoSort(t *testing.T) {
	var hashes [][]byte
	var hashescp [][]byte
	for i := 100000; i < 100100; i++ {
		ii := make([]byte, 8)
		enc.MachineEndian.PutUint64(ii, uint64(i))
		h := hash.Hash256b(ii)
		hashes = append(hashes, h[:])
		hashescp = append(hashescp, h[:])
	}

	Sort(hashes, 481)

	same := true
	for i, s := range hashes {
		if !bytes.Equal(s, hashescp[i]) {
			same = false
			break
		}
	}
	assert.False(t, same)
}
