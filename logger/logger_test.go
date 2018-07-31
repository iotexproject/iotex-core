// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package logger

import "testing"

func TestLogger(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			Log().Msg("r == nil")
		}
	}()

	Log().Msg("test")
	Debug().Str("test string", "test").Float32("test float", 1.2345).Msg("Test Msg")
	Info().Str("test string", "test").Float32("test float", 1.2345).Msg("Test Msg")
	Warn().Str("test string", "test").Float32("test float", 1.2345).Msg("Test Msg")
	Error().Str("test string", "test").Float32("test float", 1.2345).Msg("Test Msg")
	Panic().Str("test string", "test").Float32("test float", 1.2345).Msg("Test Msg")
}
