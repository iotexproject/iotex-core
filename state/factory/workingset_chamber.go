// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/go-pkgs/hash"
)

type (
	WorkingSetChamber interface {
		GetWorkingSet(any) *workingSet
		PutWorkingSet(hash.Hash256, *workingSet)
		Clear()
	}

	chamber struct {
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
}

func (cmb *chamber) Clear() {
	cmb.ws.Clear()
	cmb.header.Clear()
}
