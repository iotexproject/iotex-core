// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// A warrper for Zerolog (https://github.com/rs/zerolog)
//
// Package log provides a global logger for zerolog.
// derived from https://github.com/rs/zerolog/blob/master/log/log.go
// putting here to get a better integration

package log

import (
	"encoding/hex"

	"go.uber.org/zap"
)

// Hex creates a zap field which convert binary to hex.
func Hex(k string, d []byte) zap.Field {
	return zap.String(k, hex.EncodeToString(d))
}
