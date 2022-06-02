// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hdwallet

import (
	"github.com/pkg/errors"
)

// Errors
var (
	ErrPasswdNotMatch = errors.New("password doesn't match")
)

// DefaultRootDerivationPath for iotex
// https://github.com/satoshilabs/slips/blob/master/slip-0044.md
const DefaultRootDerivationPath = "m/44'/304'"

var _hdWalletConfigFile string
