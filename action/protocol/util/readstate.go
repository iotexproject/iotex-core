// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/address"
)

// ReadState reads the state by calling the protocol methods
func ReadState(
	ctx context.Context,
	sm protocol.StateManager,
	p protocol.Protocol,
	method []byte,
	args ...[]byte,
) ([]byte, error) {
	// Make it general
	switch p := p.(type) {
	case *rewarding.Protocol:
		switch string(method) {
		case "UnclaimedBalance":
			if len(args) != 1 {
				return nil, errors.Errorf("invalid number of arguments %d", len(args))
			}
			addr, err := address.FromString(string(args[0]))
			if err != nil {
				return nil, err
			}
			balance, err := p.UnclaimedBalance(ctx, sm, addr)
			return []byte(balance.String()), nil
		default:
			return nil, errors.New("corresponding method isn't found")
		}
	default:
		return nil, errors.New("corresponding protocol isn't found")
	}
}
