// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// TestWorkingSet_Mint_DoesNotModifyPooledTxContainer verifies that minting a
// TX_CONTAINER action puts an unfolded copy into the block, and leaves the
// envelope held by the actpool untouched while it is read concurrently
func TestWorkingSet_Mint_DoesNotModifyPooledTxContainer(t *testing.T) {
	require := require.New(t)
	const (
		chainID      = 1
		evmNetworkID = 4689
	)
	registry := protocol.NewRegistry()
	noDeposit := func(context.Context, protocol.StateManager, *big.Int, ...protocol.DepositOption) ([]*action.TransactionLog, error) {
		return nil, nil
	}
	require.NoError(account.NewProtocol(noDeposit).Register(registry))
	cfg := Config{
		Chain:   blockchain.DefaultConfig,
		Genesis: genesis.TestDefault(),
	}
	sender := identityset.Address(28)
	cfg.Genesis.InitBalanceMap[sender.String()] = "100000000000000000000"
	f, err := NewStateDB(cfg, db.NewMemKVStore(), RegistryStateDBOption(registry))
	require.NoError(err)
	startCtx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), cfg.Genesis),
		protocol.BlockCtx{},
	)
	require.NoError(f.Start(startCtx))
	defer func() {
		require.NoError(f.Stop(startCtx))
	}()

	to := common.BytesToAddress(identityset.Address(29).Bytes())
	tx := types.MustSignNewTx(identityset.PrivateKey(28).EcdsaPrivateKey().(*ecdsa.PrivateKey),
		types.NewEIP155Signer(big.NewInt(evmNetworkID)), &types.LegacyTx{
			Nonce:    1,
			GasPrice: big.NewInt(1000000000000),
			Gas:      testutil.TestGasLimit,
			To:       &to,
			Value:    big.NewInt(1),
		})
	raw, err := tx.MarshalBinary()
	require.NoError(err)
	core, err := action.EthRawToContainer(chainID, hex.EncodeToString(raw))
	require.NoError(err)
	_, sig, pubkey, err := action.ExtractTypeSigPubkey(tx)
	require.NoError(err)
	selp, err := (&action.Deserializer{}).SetEvmNetworkID(evmNetworkID).ActionToSealedEnvelope(&iotextypes.Action{
		Core:         core,
		SenderPubKey: pubkey.Bytes(),
		Signature:    sig,
		Encoding:     iotextypes.Encoding_TX_CONTAINER,
	})
	require.NoError(err)
	pooledHash, err := selp.Hash()
	require.NoError(err)

	ctrl := gomock.NewController(t)
	ap := mock_actpool.NewMockActPool(ctrl)
	ap.EXPECT().BundlePool().Return(nil).Times(1)
	ap.EXPECT().PendingActionMap().Return(map[string][]*action.SealedEnvelope{
		sender.String(): {selp},
	}).Times(1)

	ctx := protocol.WithBlockCtx(context.Background(),
		protocol.BlockCtx{
			BlockHeight: uint64(1),
			Producer:    identityset.Address(27),
			GasLimit:    testutil.TestGasLimit * 100000,
		})
	ctx = protocol.WithBlockchainCtx(
		genesis.WithGenesisContext(ctx, cfg.Genesis),
		protocol.BlockchainCtx{ChainID: chainID, EvmNetworkID: evmNetworkID},
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))

	// read the pooled envelope concurrently, as the API does
	var (
		wg   sync.WaitGroup
		stop = make(chan struct{})
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = selp.Encoding()
				_ = selp.Proto()
			}
		}
	}()
	blk, err := f.Mint(ctx, ap, identityset.PrivateKey(27))
	close(stop)
	wg.Wait()
	require.NoError(err)

	// the block holds the unfolded action
	var found bool
	for _, act := range blk.Actions {
		if _, ok := act.Envelope.(action.TxContainer); ok {
			require.Fail("block contains a tx container")
		}
		if act.SenderAddress().String() == sender.String() {
			found = true
			require.Equal(uint32(iotextypes.Encoding_ETHEREUM_EIP155), act.Encoding())
		}
	}
	require.True(found)
	// the pooled envelope is not modified
	_, ok := selp.Envelope.(action.TxContainer)
	require.True(ok)
	require.Equal(uint32(iotextypes.Encoding_TX_CONTAINER), selp.Encoding())
	h, err := selp.Hash()
	require.NoError(err)
	require.Equal(pooledHash, h)
}
