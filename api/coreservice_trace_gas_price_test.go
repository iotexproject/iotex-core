// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// TestTraceGasPriceOpcode calls, with a dynamic-fee tx, a contract that stores
// what the GASPRICE opcode reports and returns it, then traces the call. The
// trace must see the value block execution saw: the fee cap before the next
// hardfork, and the effective gas price min(feeCap, baseFee+tipCap) from it,
// which needs the traced block's base fee.
func TestTraceGasPriceOpcode(t *testing.T) {
	var (
		sender = identityset.Address(28)
		feeCap = new(big.Int).SetUint64(2 * action.InitialBaseFee)
		tipCap = big.NewInt(1)
		// GASPRICE DUP1 PUSH0 SSTORE PUSH0 MSTORE PUSH1 0x20 PUSH0 RETURN
		runtime = []byte{0x3a, 0x80, 0x5f, 0x55, 0x5f, 0x52, 0x60, 0x20, 0x5f, 0xf3}
		// PUSH10 runtime PUSH0 MSTORE PUSH1 10 PUSH1 22 RETURN
		initCode = append(append([]byte{0x69}, runtime...), 0x5f, 0x52, 0x60, 0x0a, 0x60, 0x16, 0xf3)
	)
	for _, tt := range []struct {
		name       string
		gateHeight uint64
		effective  bool
	}{
		{"before the next hardfork", math.MaxUint64, false},
		{"from the next hardfork", 1, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			cfg := newConfig()
			cfg.genesis.YapBlockHeight = 1 // enable dynamic-fee txs and PUSH0
			testutil.NormalizeGenesisHeights(&cfg.genesis.Blockchain)
			cfg.genesis.ToBeEnabledBlockHeight = tt.gateHeight
			// tracing replays the block on the state of its parent, which
			// only an archive node keeps
			cfg.chain.HistoryIndexPath = t.TempDir()
			bc, dao, indexer, bfIndexer, sf, ap, registry, bfIndexFile, err := setupChain(cfg)
			require.NoError(err)
			defer testutil.CleanupPath(bfIndexFile)
			ctx := context.Background()
			require.NoError(bc.Start(ctx))
			defer func() {
				require.NoError(bc.Stop(ctx))
			}()
			svr, err := newCoreService(cfg.api, bc, nil, sf, dao, indexer, bfIndexer, ap, registry)
			require.NoError(err)

			// a dynamic-fee tx only enters the chain in Ethereum encoding
			evmNetworkID := bc.EvmNetworkID()
			signTx := func(nonce uint64, to *common.Address, data []byte) *action.SealedEnvelope {
				tx := types.MustSignNewTx(
					identityset.PrivateKey(28).EcdsaPrivateKey().(*ecdsa.PrivateKey),
					types.NewCancunSigner(new(big.Int).SetUint64(uint64(evmNetworkID))),
					&types.DynamicFeeTx{
						Nonce:     nonce,
						GasTipCap: tipCap,
						GasFeeCap: feeCap,
						Gas:       200_000,
						To:        to,
						Value:     big.NewInt(0),
						Data:      data,
					})
				raw, err := tx.MarshalBinary()
				require.NoError(err)
				_, sig, pubkey, err := action.ExtractTypeSigPubkey(tx)
				require.NoError(err)
				core, err := action.EthRawToContainer(bc.ChainID(), hex.EncodeToString(raw))
				require.NoError(err)
				selp, err := (&action.Deserializer{}).SetEvmNetworkID(evmNetworkID).ActionToSealedEnvelope(&iotextypes.Action{
					Core:         core,
					SenderPubKey: pubkey.Bytes(),
					Signature:    sig,
					Encoding:     iotextypes.Encoding_TX_CONTAINER,
				})
				require.NoError(err)
				return selp
			}
			nonce, err := ap.GetPendingNonce(sender.String())
			require.NoError(err)
			contract := crypto.CreateAddress(common.BytesToAddress(sender.Bytes()), nonce)
			deploy := signTx(nonce, nil, initCode)
			call := signTx(nonce+1, &contract, nil)
			require.NoError(ap.Add(ctx, deploy))
			require.NoError(ap.Add(ctx, call))
			blk, err := bc.MintNewBlock(testutil.TimestampNow())
			require.NoError(err)
			require.NoError(bc.CommitBlock(blk))
			require.Len(blk.Actions, 3) // deploy, call and the grant reward
			require.NotNil(blk.BaseFee())

			want := feeCap
			if tt.effective {
				want = new(big.Int).Add(blk.BaseFee(), tipCap)
				require.Negative(want.Cmp(feeCap))
			}
			wantRet := common.BigToHash(want).Bytes()

			// slot 0 holds what GASPRICE reported during block execution
			contractAddr, err := address.FromBytes(contract.Bytes())
			require.NoError(err)
			stored, err := svr.ReadContractStorageAt(ctx, contractAddr, make([]byte, 32), blk.Height())
			require.NoError(err)
			require.Equal(wantRet, stored)

			h, err := call.Hash()
			require.NoError(err)
			retval, receipt, _, err := svr.TraceTransaction(ctx, hex.EncodeToString(h[:]), nil)
			require.NoError(err)
			require.Equal(uint64(1), receipt.Status)
			require.Equal(wantRet, retval)

			retvals, receipts, _, err := svr.TraceBlockByNumber(ctx, blk.Height(), nil)
			require.NoError(err)
			require.Equal(uint64(1), receipts[1].Status)
			require.Equal(wantRet, retvals[1])
		})
	}
}
