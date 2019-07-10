// Copyright (c) 2019 IoTeX
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

// StringMessage is for implementing string as interface Message
type StringMessage string

func (m StringMessage) String() string {
	if Format == "" {
		return string(m)
	}
	return FormatString(Result, m)
}

// FormatString returns Output as string in certain format
func FormatString(t MessageType, m Message) string {
	switch Format {
	default: // json is default
		out := Output{
			MessageType: t,
			Message:     m,
		}
		byteAsJSON, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			log.Panic(err)
		}
		return fmt.Sprintln(string(byteAsJSON))
	}
}

// PrintError prints error message in format, and returns golang error when using default output
func PrintError(code ErrorCode, info string) error {
	errMessage := ErrorMessage{Code: code, Info: info}
	if Format == "" {
		return fmt.Errorf(errMessage.String())
	}
	fmt.Println(errMessage.String())
	return nil
}
