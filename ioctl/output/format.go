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
	"regexp"
	"strconv"
	"strings"
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

func newError(code ErrorCode, info string, pre error) ErrorMessage {
	// find out previous ErrorMessage and recompose it
	if pre != nil {
		errParts := strings.Split(pre.Error(), ", ")
		if len(errParts) >= 2 {
			if !strings.Contains(errParts[0], " ") {
				ok, _ := regexp.MatchString(`^[1-9]\d|0$`, errParts[0])
				if ok {
					preCode, err := strconv.Atoi(errParts[0])
					if err == nil {
						if code == 0 {
							code = ErrorCode(preCode)
						}
						pre = fmt.Errorf(strings.Join(errParts[1:], ", "))
					}
				}
			}
		}
		if len(info) != 0 {
			info += ": "
		}
		info += pre.Error()
	}
	return ErrorMessage{Code: code, Info: info}
}

// NewError and returns golang error that contains Error Message
func NewError(code ErrorCode, newInfo string, pre error) error {
	message := newError(code, newInfo, pre)
	return fmt.Errorf(message.String())
}

// PrintError prints Error Message in format
func PrintError(err error) error {
	if err == nil {
		return nil
	}
	message := newError(0, "", err)
	if Format == "" {
		return fmt.Errorf(message.String())
	}
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
