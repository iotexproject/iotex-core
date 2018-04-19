// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package txvm

// ErrorCode identifies a kind of txvm error.
type ErrorCode int

// These constants are used to identify a specific Error.
const (
	// ErrInternal is returned if internal consistency checks fail.
	ErrInternal ErrorCode = iota

	// Errors emitted by builder
	// ErrUnsupportedOpcode ...
	ErrUnsupportedOpcode
	// ErrInvalidOpcode ...
	ErrInvalidOpcode
	// ErrInvalidOpdata ...
	ErrInvalidOpdata

	// Errors emitted by IVM
	// ErrEqualVerify...
	ErrEqualVerify
	// ErrInvalidStackOperation ...
	ErrInvalidStackOperation
)

// ScriptError defines the struct of script error
type ScriptError struct {
	ErrorCode ErrorCode
	Desc      string
}

func (e ScriptError) Error() string {
	return e.Desc
}

func scriptError(c ErrorCode, desc string) ScriptError {
	return ScriptError{c, desc}
}
