// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package output

// OutputFormat is the target of output-format flag
var OutputFormat string

// ErrorCode is the code of error
type ErrorCode int

const (
	// NoError used when no error is happened
	NoError ErrorCode = iota
	// UndefinedError used when an error cat't be classified 
	UndefinedError
	// NetworkError used when an network error is happened
	NetworkError
	// APIError used when an API error is happened
	APIError
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

// Output is used for format output
type Output struct {
	Error   Error   `json:"error"`
	Message Message `json:"message"`
}

// Error is the error part of Output
type Error struct {
	Code ErrorCode `json:"code"`
	Info string    `json:"info"`
}

// Message is the message part of Output
type Message interface {
}
