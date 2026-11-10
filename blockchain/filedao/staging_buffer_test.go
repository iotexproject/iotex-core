// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

func TestStagingBufferSerializeWith(t *testing.T) {
	r := require.New(t)
	const size, start = 4, 1
	builder := block.NewTestingBuilder()
	h := hash.ZeroHash256
	stores := make([]*block.Store, 0, 2*size)
	for i := uint64(start); i < start+2*size; i++ {
		blk := createTestingBlock(builder, i, h)
		stores = append(stores, &block.Store{Block: blk, Receipts: blk.Receipts})
		h = blk.HashBlock()
	}
	// the buffer holds the first round, and the second round up to its last slot
	buf := newStagingBuffer(size, start)
	for _, s := range stores[:2*size-1] {
		_, err := buf.Put(s.Block.Height(), s)
		r.NoError(err)
	}
	slots := append([]*block.Store{}, buf.buffer...)
	r.Same(stores[size-1], slots[size-1])

	last := stores[2*size-1]
	r.True(buf.isLastSlot(last.Block.Height()))
	ser, err := buf.serializeWith(last.Block.Height(), last)
	r.NoError(err)

	// the output has the new block in its slot
	pb, err := block.DeserializeBlockStoresPb(ser)
	r.NoError(err)
	r.Len(pb.BlockStores, size)
	for i, want := range stores[size:] {
		r.True(proto.Equal(want.ToProtoWithoutSidecar(), pb.BlockStores[i]), "slot %d", i)
	}
	// and the buffer is unchanged
	for i := range slots {
		r.Same(slots[i], buf.buffer[i], "slot %d", i)
	}

	_, err = buf.serializeWith(start-1, last)
	r.ErrorIs(err, ErrNotSupported)
}
