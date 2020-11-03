// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"testing"
)

func TestEnc(t *testing.T) {
	data := []struct {
		input string
		key   string
	}{
		{"Foo", "Boo"},
		{"Bar", "Car"},
		{"Bar", ""},
		{"", "Car"},
		{"Long input with more than 16 characters", "Car"},
	}
	for _, d := range data {
		enc, err := EncryptString(d.input, d.key)
		if err != nil {
			t.Errorf("Unable to encrypt '%v' with key '%v': %v", d.input, d.key, err)
			continue
		}
		dec, err := DecryptString(enc, d.key)
		if err != nil {
			t.Errorf("Unable to decrypt '%v' with key '%v': %v", enc, d.key, err)
			continue
		}
		if dec != d.input {
			t.Errorf("Decrypt Key %v\n  Input: %v\n  Expect: %v\n  Actual: %v", d.key, enc, d.input, enc)
		}
	}
}
