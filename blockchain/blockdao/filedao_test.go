// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
)

func TestChecksumNamespaceAndKeys(t *testing.T) {
	r := require.New(t)

	a := []hash.Hash256{
		// filedao
		hash.BytesToHash256([]byte(blockHashHeightMappingNS)),
		hash.BytesToHash256([]byte(systemLogNS)),
		hash.BytesToHash256(topHeightKey),
		hash.BytesToHash256(topHashKey),
		hash.BytesToHash256(hashPrefix),
		// filedao_legacy
		hash.BytesToHash256([]byte(blockNS)),
		hash.BytesToHash256([]byte(blockHeaderNS)),
		hash.BytesToHash256([]byte(blockBodyNS)),
		hash.BytesToHash256([]byte(blockFooterNS)),
		hash.BytesToHash256([]byte(receiptsNS)),
		hash.BytesToHash256(heightPrefix),
		hash.BytesToHash256(heightToFileBucket),
		// filedao_v2
		hash.BytesToHash256([]byte{_normal}),
		hash.BytesToHash256([]byte{_compressed}),
		hash.BytesToHash256([]byte{blockStoreBatchSize}),
		hash.BytesToHash256([]byte(hashDataNS)),
		hash.BytesToHash256([]byte(blockDataNS)),
		hash.BytesToHash256([]byte(stagingDataNS)),
		hash.BytesToHash256(bottomHeightKey),
	}

	checksum := crypto.NewMerkleTree(a)
	r.NotNil(checksum)
	h := checksum.HashTree()
	r.Equal("f584356aa8ad1da8cb803162f06c96ff04e4478ad8f4a2667d44cee3e79f9d94", hex.EncodeToString(h[:]))
}
