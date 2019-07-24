// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package output

import (
	"encoding/json"
	"fmt"
	"log"
)

// Format is the target of output-format flag
var Format string

// ErrorCode is the code of error
type ErrorCode int

const (
	// UndefinedError used when an error cat't be classified
	UndefinedError ErrorCode = iota
	// UpdateError used when an error occurs when running update command
	UpdateError
	// RuntimeError used when an error occurs in runtime
	RuntimeError
	// NetworkError used when an network error is happened
	NetworkError
	// APIError used when an API error is happened
	APIError
	// ValidationError used when validation is not passed
	ValidationError
	// SerializationError used when marshal or unmarshal meets error
	SerializationError
	// ReadFileError used when error occurs during reading a file
	ReadFileError
	// WriteFileError used when error occurs during writing a file
	WriteFileError
	// FlagError used when invalid flag is set
	FlagError
	// ConvertError used when fail to converting data
	ConvertError
	// CryptoError used when crypto error occurs
	CryptoError
	// AddressError used if an error is related to address
	AddressError
	// InputError used when error about input occurs
	InputError
	// KeystoreError used when an error related to keystore
	KeystoreError
	// ConfigError used when an error about config occurs
	ConfigError
	// InstantiationError used when an error during instantiation
	InstantiationError
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
	// Error represents error occured when running a command
	Error
)

// Output is used for format output
type Output struct {
	MessageType MessageType `json:"messageType"`
	Message     Message     `json:"message"`
}

// Message is the message part of output
type Message interface {
	String() string
}

// ConfirmationMessage is the struct of an Confirmation output
type ConfirmationMessage struct {
	Info    string   `json:"info"`
	Options []string `json:"options"`
}

func (m *ConfirmationMessage) String() string {
	if Format == "" {
		line := fmt.Sprintf("%s\nOptions:", m.Info)
		for _, option := range m.Options {
			line += " " + option
		}
		line += "\nQuit for anything else."
		return line
	}
	return FormatString(Confirmation, m)
}

// ErrorMessage is the struct of an Error output
type ErrorMessage struct {
	Code ErrorCode `json:"code"`
	Info string    `json:"info"`
}

func (m *ErrorMessage) String() string {
	if Format == "" {
		return fmt.Sprintf("%d, %s", m.Code, m.Info)
	}
	return FormatString(Error, m)
}

// Error implements error interface
func (m ErrorMessage) Error() string {
	return fmt.Sprintf("%d, %s", m.Code, m.Info)
}

// StringMessage is the Message for string
type StringMessage string

func (m StringMessage) String() string {
	if Format == "" {
		return string(m)
	}
	return FormatString(Result, m)
}

// Query prints query message
func (m StringMessage) Query() string {
	if Format == "" {
		return string(m)
	}
	return FormatString(Query, m)
}

// FormatString returns Output as string in certain format
func FormatString(t MessageType, m Message) string {
	out := Output{
		MessageType: t,
		Message:     m,
	}
	switch Format {
	default: // default is json
		return JSONString(out)
	}
}

// JSONString returns json string for message
func JSONString(out interface{}) string {
	byteAsJSON, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		log.Panic(err)
	}
	return fmt.Sprint(string(byteAsJSON))
}

// NewError and returns golang error that contains Error Message
// ErrorCode can pass zero only when previous error is always a format error
// that contains non-zero error code. ErrorCode passes 0 means that I want to
// use previous error's code rather than override it.
// If there is no previous error, newInfo should not be empty.
func NewError(code ErrorCode, info string, pre error) error {
	if pre == nil {
		return ErrorMessage{Code: code, Info: info}
	}
	message, ok := pre.(ErrorMessage)
	if ok {
		if code != 0 {
			// override error code
			message.Code = code
		}
		if len(info) != 0 {
			message.Info = fmt.Sprintf("%s: %s", info, message.Info)
		}
	} else {
		message = ErrorMessage{Code: code, Info: fmt.Sprintf("%s: %s", info, pre.Error())}
	}
	return message
}

// PrintError prints Error Message in format, only used at top layer of a command
func PrintError(err error) error {
	if err == nil {
		return nil
	}
	newErr := NewError(0, "", err)
	if Format == "" {
		return newErr
	}
	message := newErr.(ErrorMessage)
	fmt.Println(message.String())
	return nil
}

// PrintResult prints result message in format
func PrintResult(result string) {
	message := StringMessage(result)
	fmt.Println(message.String())
}

// PrintQuery prints query message in format
func PrintQuery(query string) {
	message := StringMessage(query)
	fmt.Println(message.Query())
}
