// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement,
// merchantability or fitness for purpose and, to the extent permitted by law,
// all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package contract

import (
	"encoding/hex"

	"github.com/iotexproject/iotex-core/ioctl/util"
)

func decodeBytecode(bytecode string) ([]byte, error) {
	return hex.DecodeString(util.TrimHexPrefix(bytecode))
}
