// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import "github.com/iotexproject/iotex-core/state"

type noncer interface {
	Nonce() uint64
}

// SetNonce sets nonce for account
func SetNonce(i noncer, state *state.Account) {
	if i.Nonce() > state.Nonce {
		state.Nonce = i.Nonce()
	}
}
