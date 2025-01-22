// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

type (
	WorkingSetChamber interface {
		GetWorkingSet(any) *workingSet
		PutWorkingSet(hash.Hash256, *workingSet)
		OngoingBlockHeight() uint64
		GetBlockHeader(uint64) *block.Header
		PutBlockHeader(*block.Header)
		Clear()
	}

	chamber struct {
		height uint64
		ws     cache.LRUCache
		header cache.LRUCache
	}
)

func newWorkingsetChamber(size int) WorkingSetChamber {
	return &chamber{
		ws:     cache.NewThreadSafeLruCache(size),
		header: cache.NewThreadSafeLruCache(size),
	}
}

func (cmb *chamber) GetWorkingSet(key any) *workingSet {
	if data, ok := cmb.ws.Get(key); ok {
		return data.(*workingSet)
	}
	return nil
}

func (cmb *chamber) PutWorkingSet(key hash.Hash256, ws *workingSet) {
	cmb.ws.Add(key, ws)
	cmb.ws.Add(ws.height, ws)
	if ws.height > cmb.height {
		cmb.height = ws.height
	}
}

func (cmb *chamber) OngoingBlockHeight() uint64 {
	return cmb.height
}

func (cmb *chamber) GetBlockHeader(height uint64) *block.Header {
	if data, ok := cmb.header.Get(height); ok {
		return data.(*block.Header)
	}
	return nil
}

func (cmb *chamber) PutBlockHeader(h *block.Header) {
	cmb.header.Add(h.Height(), h)
}

func (cmb *chamber) Clear() {
	cmb.ws.Clear()
	cmb.header.Clear()
}
