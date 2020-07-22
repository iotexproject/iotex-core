// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"github.com/pkg/errors"
)

// Errors
var (
	ErrNoName        = errors.New("name does not exist")
	ErrTypeAssertion = errors.New("failed type assertion")
)

type dock struct {
	dirty     bool
	value     map[string]interface{}
	protocols map[string]bool
}

// NewDock returns a new dock
func NewDock() Dock {
	return &dock{
		value:     make(map[string]interface{}),
		protocols: make(map[string]bool),
	}
}

func (d *dock) ProtocolDirty(name string) bool {
	return d.protocols[name]
}

func (d *dock) Load(name string, v interface{}) error {
	d.value[name] = v
	d.protocols[name] = true
	return nil
}

func (d *dock) Unload(name string) (interface{}, error) {
	if v, hit := d.value[name]; hit {
		return v, nil
	}
	return nil, errors.Wrapf(ErrNoName, "name %s", name)
}

func (d *dock) Push() error {
	return nil
}

func (d *dock) Reset() {
	d.value = nil
	d.protocols = nil
	d.value = make(map[string]interface{})
	d.protocols = make(map[string]bool)
}

// UnloadAndAssertBytes read from dock and assert the value is []byte
func UnloadAndAssertBytes(sm StateManager, name string) ([]byte, error) {
	v, err := sm.Unload(name)
	if err != nil {
		return nil, err
	}

	ser, ok := v.([]byte)
	if !ok {
		return nil, errors.Wrap(ErrTypeAssertion, "failed to unload data from dock, expecting []byte")
	}
	return ser, nil
}
