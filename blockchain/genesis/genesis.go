// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package genesis

const (
	// BlockGasLimit is the total gas limit could be consumed in a block
	BlockGasLimit = uint64(200000000)
	// ActionGasLimit is the per action gas limit cap
	ActionGasLimit = BlockGasLimit / 10
)
