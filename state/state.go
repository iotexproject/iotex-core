// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"bytes"
	"encoding/gob"
	"reflect"

	"github.com/pkg/errors"
)

var (
	// ErrStateSerialization is the error that the state marshaling is failed
	ErrStateSerialization = errors.New("failed to marshal state")

	// ErrStateDeserialization is the error that the state un-marshaling is failed
	ErrStateDeserialization = errors.New("failed to unmarshal state")
)

// State is the interface, which defines the common methods for state struct to be handled by state factory
type State interface {
	Serialize() ([]byte, error)
	Deserialize(data []byte) error
}

// GobBasedSerialize serializes a state into bytes via gob
func GobBasedSerialize(state State) ([]byte, error) {
	var buf bytes.Buffer
	e := gob.NewEncoder(&buf)
	if err := e.Encode(state); err != nil {
		return nil, errors.Wrapf(ErrStateSerialization, "error when serializing %s state", reflect.TypeOf(state).String())
	}
	return buf.Bytes(), nil
}

// GobBasedDeserialize deserialize a state from bytes via gob
func GobBasedDeserialize(state State, data []byte) error {
	buf := bytes.NewBuffer(data)
	e := gob.NewDecoder(buf)
	if err := e.Decode(state); err != nil {
		return errors.Wrapf(ErrStateDeserialization, "error when deserializing %s state", reflect.TypeOf(state).String())
	}
	return nil
}
