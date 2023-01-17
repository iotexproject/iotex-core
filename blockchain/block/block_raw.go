// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

import "google.golang.org/protobuf/proto"

// BlockRaw is block with raw/proto data
type BlockRaw struct {
	blk   *Block
	proto proto.Message
}

func (b *BlockRaw) Proto() proto.Message {
	return b.proto
}

func (b *BlockRaw) Block() (*Block, error) {
	// TODO: if blk == nil, de-serialize proto into blk
	return b.blk, nil
}
