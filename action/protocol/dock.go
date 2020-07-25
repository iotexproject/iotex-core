// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/state"
)

// Errors
var (
	ErrNoName        = errors.New("name does not exist")
	ErrTypeAssertion = errors.New("failed type assertion")
)

type dock struct {
	stash batch.KVStoreCache
	dirty map[string]bool
}

// NewDock returns a new dock
func NewDock() Dock {
	return &dock{
		stash: batch.NewKVCache(),
		dirty: make(map[string]bool),
	}
}

func (d *dock) ProtocolDirty(name string) bool {
	return d.dirty[name]
}

func (d *dock) Load(ns, key string, v interface{}) error {
	ser, err := state.Serialize(v)
	if err != nil {
		return err
	}
	d.dirty[ns] = true
	d.stash.Write(stashKey(ns, key), ser)
	return nil
}

func (d *dock) Unload(ns, key string, v interface{}) error {
	ser, err := d.stash.Read(stashKey(ns, key))
	if err != nil {
		return ErrNoName
	}
	return state.Deserialize(v, ser)
}

func (d *dock) Reset() {
	d.dirty = nil
	d.dirty = make(map[string]bool)
	d.stash.Clear()
}

func stashKey(ns, key string) hash.Hash160 {
	hk := hash.Hash160b([]byte(key))
	return hash.Hash160b(append([]byte(ns), hk[:]...))
}
