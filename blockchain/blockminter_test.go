// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestMintNewBlock(t *testing.T) {
	require := require.New(t)
	cfg := config.Default

	sf, err := factory.NewFactory(cfg, factory.DefaultTrieOption())
	require.NoError(err)
	minter := minter{sf: sf, minterPrivateKey: identityset.PrivateKey(2)}

	addr0 := identityset.Address(27).String()
	tsf0, err := testutil.SignedTransfer(addr0, identityset.PrivateKey(0), 1, big.NewInt(90000000), nil, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	require.NoError(err)
	accMap := make(map[string][]action.SealedEnvelope)
	accMap[identityset.Address(0).String()] = []action.SealedEnvelope{tsf0}

	ctx := protocol.WithBlockchainCtx(
		context.Background(),
		protocol.BlockchainCtx{
			Genesis: config.Default.Genesis,
			Tip: protocol.TipInfo{
				Height: 0,
				Hash:   hash.ZeroHash256,
			},
		},
	)
	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight:    1,
			BlockTimeStamp: testutil.TimestampNow(),
		},
	)
	blk, err := minter.Mint(ctx, accMap)
	require.NotNil(blk)
	require.NoError(err)
}
