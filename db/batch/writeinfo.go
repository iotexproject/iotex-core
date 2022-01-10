// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package batch

const (
	// Put indicate the type of write operation to be Put
	Put WriteType = iota
	// Delete indicate the type of write operation to be Delete
	Delete
)

type (
	// WriteType is the type of write
	WriteType uint8

	// WriteInfo is the struct to store Put/Delete operation info
	WriteInfo struct {
		writeType   WriteType
		namespace   string
		key         []byte
		value       []byte
		errorFormat string
		errorArgs   interface{}
	}

	// WriteInfoFilter filters a write
	WriteInfoFilter func(wi *WriteInfo) bool

	// WriteInfoSerialize serializes a write to bytes
	WriteInfoSerialize func(wi *WriteInfo) []byte

	// WriteInfoTranslate translates a write info
	WriteInfoTranslate func(wi *WriteInfo) *WriteInfo
)

// NewWriteInfo creates a new write info
func NewWriteInfo(
	writeType WriteType,
	namespace string,
	key,
	value []byte,
	errorFormat string,
	errorArgs interface{},
) *WriteInfo {
	return &WriteInfo{
		writeType:   writeType,
		namespace:   namespace,
		key:         key,
		value:       value,
		errorFormat: errorFormat,
		errorArgs:   errorArgs,
	}
}

// Namespace returns the namespace of a write info
func (wi *WriteInfo) Namespace() string {
	return wi.namespace
}

// WriteType returns the type of a write info
func (wi *WriteInfo) WriteType() WriteType {
	return wi.writeType
}

// Key returns a copy of key
func (wi *WriteInfo) Key() []byte {
	key := make([]byte, len(wi.key))
	copy(key, wi.key)

	return key
}

// Value returns a copy of value
func (wi *WriteInfo) Value() []byte {
	value := make([]byte, len(wi.value))
	copy(value, wi.value)

	return value
}

// ErrorFormat returns the error format
func (wi *WriteInfo) ErrorFormat() string {
	return wi.errorFormat
}

// ErrorArgs returns the error args
func (wi *WriteInfo) ErrorArgs() interface{} {
	return wi.errorArgs
}

// Serialize serializes the write info
func (wi *WriteInfo) Serialize() []byte {
	lenNamespace, lenKey, lenValue := len(wi.namespace), len(wi.key), len(wi.value)
	bytes := make([]byte, 1+lenNamespace+lenKey+lenValue)
	bytes[0] = byte(wi.writeType)
	copy(bytes[1:], []byte(wi.namespace))
	copy(bytes[1+lenNamespace:], wi.key)
	copy(bytes[1+lenNamespace+lenKey:], wi.value)
	return bytes
}

// SerializeWithoutWriteType serializes the write info without write type
func (wi *WriteInfo) SerializeWithoutWriteType() []byte {
	lenNamespace, lenKey, lenValue := len(wi.namespace), len(wi.key), len(wi.value)
	bytes := make([]byte, lenNamespace+lenKey+lenValue)
	copy(bytes[0:], []byte(wi.namespace))
	copy(bytes[lenNamespace:], wi.key)
	copy(bytes[lenNamespace+lenKey:], wi.value)
	return bytes
}
