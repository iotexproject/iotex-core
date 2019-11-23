// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state"
)

// createGenesisStates initialize the genesis states
func createGenesisStates(ctx context.Context, ws WorkingSet) error {
	if raCtx, ok := protocol.GetRunActionsCtx(ctx); ok {
		for _, p := range raCtx.Registry.All() {
			if gsc, ok := p.(protocol.GenesisStateCreator); ok {
				if err := gsc.CreateGenesisStates(ctx, ws); err != nil {
					return errors.Wrap(err, "failed to create genesis states for protocol")
				}
			}
		}
	}
	_ = ws.UpdateBlockLevelInfo(0)

	return nil
}

// CreateTestAccount adds a new account with initial balance to the factory for test usage
func CreateTestAccount(sf Factory, cfg config.Config, registry *protocol.Registry, addr string, init *big.Int) (*state.Account, error) {
	gasLimit := cfg.Genesis.BlockGasLimit
	if sf == nil {
		return nil, errors.New("empty state factory")
	}

	ws, err := sf.NewWorkingSet()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create clean working set")
	}

	account, err := accountutil.LoadOrCreateAccount(ws, addr, init)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create new account %s", addr)
	}

	callerAddr, err := address.FromString(addr)
	if err != nil {
		return nil, err
	}

	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			GasLimit:   gasLimit,
			Caller:     callerAddr,
			ActionHash: hash.ZeroHash256,
			Nonce:      0,
			Registry:   registry,
		})
	if _, err = ws.RunActions(ctx, 0, nil); err != nil {
		return nil, errors.Wrap(err, "failed to run the account creation")
	}

	if err = sf.Commit(ws); err != nil {
		return nil, errors.Wrap(err, "failed to commit the account creation")
	}

	return account, nil
}
