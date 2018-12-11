// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"github.com/pkg/errors"
)

var (
	// ErrStateSerialization is the error that the state marshaling is failed
	ErrStateSerialization = errors.New("failed to marshal state")

	// ErrStateDeserialization is the error that the state un-marshaling is failed
	ErrStateDeserialization = errors.New("failed to unmarshal state")

	// ErrStateNotExist is the error that the state does not exist
	ErrStateNotExist = errors.New("state does not exist")
)

// State is the interface, which defines the common methods for state struct to be handled by state factory
type State interface {
	Serialize() ([]byte, error)
	Deserialize(data []byte) error
}

// Serializer has Serialize method to serialize struct to binary data.
type Serializer interface {
	Serialize() ([]byte, error)
}

// Deserializer has Deserialize method to deserialize binary data to struct.
type Deserializer interface {
	Deserialize(data []byte) error
}

// Serialize check if input is Serializer, if it is, use the input's Serialize method, otherwise use Gob.
func Serialize(d interface{}) ([]byte, error) {
	if s, ok := d.(Serializer); ok {
		return s.Serialize()
	}
	panic("data holder doesn't implement Serializer interface!")
}

// Deserialize check if input is Deserializer, if it is, use the input's Deserialize method, otherwise use Gob.
func Deserialize(x interface{}, data []byte) error {
	if s, ok := x.(Deserializer); ok {
		return s.Deserialize(data)
	}
	panic("data holder doesn't implement Deserializer interface!")
}
