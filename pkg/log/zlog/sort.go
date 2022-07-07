// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package zlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	jpatch "github.com/evanphx/json-patch/v5"
	"github.com/rs/zerolog"
)

const (
	// custom define the field to be sorted
	_ioAddrFieldName = "ioAddr"
)

type (
	// partsSortWriter represents custom field sorting
	partsSortWriter struct {
		f *os.File
	}
	partsSort struct {
		Time    string `json:"time"`
		Level   string `json:"level"`
		Caller  string `json:"caller"`
		Message string `json:"message"`
		IoAddr  string `json:"ioAddr"`
	}
)

// Write implements interface io.Write by sorting custom fields then writing into file
func (c *partsSortWriter) Write(p []byte) (int, error) {
	var evt map[string]interface{}
	d := json.NewDecoder(bytes.NewReader(p))
	d.UseNumber()
	if err := d.Decode(&evt); err != nil {
		return 0, fmt.Errorf("cannot decode event: %v", err)
	}

	var partsSort partsSort
	defaultPartsSort := defaultPartsSort()
	for _, p := range defaultPartsSort {
		if s, ok := evt[p].(string); ok {
			switch p {
			case zerolog.TimestampFieldName:
				partsSort.Time = s
			case zerolog.LevelFieldName:
				partsSort.Level = s
			case zerolog.CallerFieldName:
				partsSort.Caller = s
			case zerolog.MessageFieldName:
				partsSort.Message = s
			case _ioAddrFieldName:
				partsSort.IoAddr = s
			}
		}
	}
	var bufSort bytes.Buffer
	if err := json.NewEncoder(&bufSort).Encode(partsSort); err != nil {
		return 0, fmt.Errorf("cannot encode partsSort: %v", err)
	}

	for field := range evt {
		switch field {
		case zerolog.TimestampFieldName,
			zerolog.LevelFieldName,
			zerolog.CallerFieldName,
			zerolog.MessageFieldName,
			_ioAddrFieldName:
			delete(evt, field)
		}
	}
	var bufEvt bytes.Buffer
	if err := json.NewEncoder(&bufEvt).Encode(evt); err != nil {
		return 0, fmt.Errorf("cannot encode evt: %v", err)
	}
	mergeBytes, err := jpatch.MergePatch(bufSort.Bytes(), bufEvt.Bytes())
	if err != nil {
		return 0, fmt.Errorf("cannot merge partsSort and evt: %v", err)
	}

	return c.f.Write(append(mergeBytes, []byte("\n")...))
}

// defaultPartsSort returns custom field sorting
func defaultPartsSort() []string {
	return []string{
		zerolog.TimestampFieldName,
		zerolog.LevelFieldName,
		zerolog.CallerFieldName,
		zerolog.MessageFieldName,
		_ioAddrFieldName,
	}
}
