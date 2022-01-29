// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

// SerializableBytes defines a type of serializable bytes
type SerializableBytes []byte

// Serialize copies and return bytes
func (sb SerializableBytes) Serialize() ([]byte, error) {
	data := make([]byte, len(sb))
	copy(data, sb)

	return data, nil
}

// Deserialize copies data into bytes
func (sb *SerializableBytes) Deserialize(data []byte) error {
	*sb = make([]byte, len(data))
	copy(*sb, data)

	return nil
}
