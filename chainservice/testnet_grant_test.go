// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestCheckTestnetGrants(t *testing.T) {
	grants := []genesis.TestnetGrant{{
		Height:     100,
		Recipients: []genesis.GrantRecipient{{Address: identityset.Address(1).String(), Amount: "1"}},
	}}

	for _, c := range []struct {
		name         string
		chainID      uint32
		evmNetworkID uint32
		grants       []genesis.TestnetGrant
		mainnetCfg   bool
		errMsg       string
	}{
		{
			name:         "no grants on mainnet ids",
			chainID:      1,
			evmNetworkID: 4689,
		},
		{
			name:         "grants on testnet ids",
			chainID:      2,
			evmNetworkID: 4690,
			grants:       grants,
		},
		{
			name:         "grants on mainnet chain id",
			chainID:      1,
			evmNetworkID: 4690,
			grants:       grants,
			errMsg:       "must not be used on mainnet",
		},
		{
			name:         "grants on mainnet evm network id",
			chainID:      2,
			evmNetworkID: 4689,
			grants:       grants,
			errMsg:       "must not be used on mainnet",
		},
		{
			// Genesis built as a literal never passes through genesis.New's
			// validate(), so the build-time check has to re-run it.
			name:         "invalid grant on testnet ids",
			chainID:      2,
			evmNetworkID: 4690,
			grants: []genesis.TestnetGrant{{
				Height:     0,
				Recipients: []genesis.GrantRecipient{{Address: identityset.Address(1).String(), Amount: "1"}},
			}},
			errMsg: "height must be non-zero",
		},
		{
			// Even with testnet IDs, the mainnet genesis config itself is
			// refused -- this is the gate that catches a mainnet genesis file
			// deployed with a doctored node config.
			name:         "mainnet genesis with testnet ids",
			chainID:      2,
			evmNetworkID: 4690,
			grants:       grants,
			mainnetCfg:   true,
			errMsg:       "must not be used on mainnet",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := require.New(t)
			cfg := config.Default
			cfg.Chain.ID = c.chainID
			cfg.Chain.EVMNetworkID = c.evmNetworkID
			if c.mainnetCfg {
				mainnet, err := genesis.New("")
				r.NoError(err)
				r.True(mainnet.IsMainnet())
				cfg.Genesis = mainnet
			} else {
				cfg.Genesis = genesis.TestDefault()
			}
			cfg.Genesis.Account.TestnetGrants = c.grants

			err := (&Builder{cfg: cfg}).checkTestnetGrants()
			if c.errMsg == "" {
				r.NoError(err)
				return
			}
			r.ErrorContains(err, c.errMsg)
		})
	}
}
