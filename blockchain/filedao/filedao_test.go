// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/pkg/compress"
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
		hash.BytesToHash256([]byte(FileV2)),
		hash.BytesToHash256([]byte{16}),
		hash.BytesToHash256([]byte(compress.Gzip)),
		hash.BytesToHash256([]byte(compress.Snappy)),
		hash.BytesToHash256([]byte(hashDataNS)),
		hash.BytesToHash256([]byte(blockDataNS)),
		hash.BytesToHash256([]byte(headerDataNs)),
		hash.BytesToHash256(fileHeaderKey),
	}

	checksum := crypto.NewMerkleTree(a)
	r.NotNil(checksum)
	h := checksum.HashTree()
	r.Equal("18747e1ac5364ce3f398e03092f159121b55166449657f65ba1f9243e8830391", hex.EncodeToString(h[:]))
}
