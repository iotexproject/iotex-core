// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/state"
)

// Errors
var (
	ErrNoName = errors.New("name does not exist")
)

type dock struct {
	stash map[string]map[string][]byte
}

// NewDock returns a new dock
func NewDock() Dock {
	return &dock{
		stash: map[string]map[string][]byte{},
	}
}

func (d *dock) ProtocolDirty(name string) bool {
	_, hit := d.stash[name]
	return hit
}

func (d *dock) Load(ns, key string, v interface{}) error {
	ser, err := state.Serialize(v)
	if err != nil {
		return err
	}

	if _, hit := d.stash[ns]; !hit {
		d.stash[ns] = map[string][]byte{}
	}
	d.stash[ns][key] = ser
	return nil
}

func (d *dock) Unload(ns, key string, v interface{}) error {
	if _, hit := d.stash[ns]; !hit {
		return ErrNoName
	}

	ser, hit := d.stash[ns][key]
	if !hit {
		return ErrNoName
	}
	return state.Deserialize(v, ser)
}

func (d *dock) Reset() {
	for k := range d.stash {
		delete(d.stash, k)
	}
}
