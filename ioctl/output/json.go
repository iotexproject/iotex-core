// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package output

var OutputFormat string

type ErrorCode int

const (
	NoError ErrorCode = iota
	Undefined_Error
	Network_Error
	API_Error
)

// MessageType marks the type of output message
type MessageType int

const (
	// Result represents the result of a command
	Result MessageType = iota
	// Confirmation represents request for confirmation
	Confirmation
	// Query represents request for answer of certain question
	Query
)

type Output struct {
	Error   Error   `json:"error"`
	Message Message `json:"message"`
}

type Error struct {
	Code ErrorCode `json:"code"`
	Info string    `json:"info"`
}

type Message interface {
}
