// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
)

func TestBlockDeserializer(t *testing.T) {
	r := require.New(t)
	bd := Deserializer{}
	blk, err := bd.FromBlockProto(&_pbBlock)
	r.NoError(err)
	body, err := bd.FromBodyProto(_pbBlock.Body)
	r.NoError(err)
	r.Equal(body, &blk.Body)

	txHash, err := blk.CalculateTxRoot()
	r.NoError(err)
	blk.Header.txRoot = txHash
	blk.Header.receiptRoot = hash.Hash256b(([]byte)("test"))
	pb := blk.ConvertToBlockPb()
	raw, err := proto.Marshal(pb)
	r.NoError(err)

	newblk, err := (&Deserializer{}).DeserializeBlock(raw)
	r.NoError(err)
	r.Equal(blk, newblk)
}
