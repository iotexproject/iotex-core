// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// payload has the method set EnvelopeBuilder.SetAction accepts
type payload interface {
	IntrinsicGas() (uint64, error)
	SanityCheck() error
	FillAction(*iotextypes.ActionCore)
}

func TestEnableOsakaEVMFlag(t *testing.T) {
	r := require.New(t)
	g := genesis.TestDefault()
	g.ToBeEnabledBlockHeight = 100
	for _, tc := range []struct {
		height uint64
		want   bool
	}{
		{99, false},
		{100, true},
	} {
		ctx := WithFeatureCtx(genesis.WithGenesisContext(
			WithBlockCtx(context.Background(), BlockCtx{BlockHeight: tc.height}), g))
		r.Equal(tc.want, MustGetFeatureCtx(ctx).EnableOsakaEVM, "height %d", tc.height)
	}
	// unscheduled by default
	ctx := WithFeatureCtx(genesis.WithGenesisContext(
		WithBlockCtx(context.Background(), BlockCtx{BlockHeight: 1 << 62}), genesis.TestDefault()))
	r.False(MustGetFeatureCtx(ctx).EnableOsakaEVM)
}

func TestValidateActionGasLimit(t *testing.T) {
	r := require.New(t)
	pre, post := FeatureCtx{}, FeatureCtx{EnableOsakaEVM: true}
	for _, gas := range []uint64{0, 21000, action.MaxTxGas} {
		r.NoError(ValidateActionGasLimit(pre, gas))
		r.NoError(ValidateActionGasLimit(post, gas))
		r.Equal(gas, CapSimulationGas(pre, gas))
		r.Equal(gas, CapSimulationGas(post, gas))
	}
	for _, gas := range []uint64{action.MaxTxGas + 1, 20_000_000, 1 << 63} {
		r.NoError(ValidateActionGasLimit(pre, gas))
		err := ValidateActionGasLimit(post, gas)
		r.ErrorIs(err, action.ErrGasLimitTooHigh)
		r.Equal(gas, CapSimulationGas(pre, gas))
		r.Equal(action.MaxTxGas, CapSimulationGas(post, gas))
	}
	r.EqualValues(1<<24, action.MaxTxGas)
}

// TestGenericValidatorEIP7825 checks the EIP-7825 cap on both validation
// entry points, one block before and at the activation height, for every kind
// of action an account can send, and that system actions are never capped.
//
// Validate is what the action pool runs at admission (and what block
// validation runs for actions not already in the pool); ValidateWithState is
// what both the proposer and every validator run on each user action of a
// block, so a block carrying an over-cap action is rejected on both sides.
func TestGenericValidatorEIP7825(t *testing.T) {
	const forkHeight = 60_000_000
	g := genesis.TestDefault()
	g.ToBeEnabledBlockHeight = forkHeight

	var (
		chainID  = uint32(1)
		senderSK = identityset.PrivateKey(28)
		to       = identityset.Address(29).String()
		balance  = new(big.Int).Lsh(big.NewInt(1), 200)
	)
	v := NewGenericValidator(nil, func(context.Context, StateReader, address.Address) (*state.Account, error) {
		acct, err := state.NewAccount()
		if err != nil {
			return nil, err
		}
		acct.Balance = balance
		return acct, nil
	})
	sign := func(t *testing.T, gas uint64, act payload) *action.SealedEnvelope {
		elp := (&action.EnvelopeBuilder{}).SetChainID(chainID).SetNonce(0).
			SetGasPrice(big.NewInt(1)).SetGasLimit(gas).SetAction(act).Build()
		selp, err := action.Sign(elp, senderSK)
		require.NoError(t, err)
		// go through the wire format, as the pool and a validating node do
		selp, err = (&action.Deserializer{}).SetEvmNetworkID(_evmNetworkID).ActionToSealedEnvelope(selp.Proto())
		require.NoError(t, err)
		return selp
	}
	// a raw Ethereum transaction, admitted as a tx container and unfolded only
	// when the block is built
	container := func(t *testing.T, gas uint64) *action.SealedEnvelope {
		toAddr := common.BytesToAddress(identityset.Address(29).Bytes())
		tx := types.MustSignNewTx(senderSK.EcdsaPrivateKey().(*ecdsa.PrivateKey),
			types.NewPragueSigner(big.NewInt(int64(_evmNetworkID))), &types.DynamicFeeTx{
				ChainID:   big.NewInt(int64(_evmNetworkID)),
				Gas:       gas,
				GasTipCap: big.NewInt(1),
				GasFeeCap: big.NewInt(1),
				To:        &toAddr,
				Value:     big.NewInt(1),
			})
		raw, err := tx.MarshalBinary()
		require.NoError(t, err)
		core, err := action.EthRawToContainer(chainID, hex.EncodeToString(raw))
		require.NoError(t, err)
		_, sig, pubkey, err := action.ExtractTypeSigPubkey(tx)
		require.NoError(t, err)
		selp, err := (&action.Deserializer{}).SetEvmNetworkID(_evmNetworkID).ActionToSealedEnvelope(&iotextypes.Action{
			Core:         core,
			SenderPubKey: pubkey.Bytes(),
			Signature:    sig,
			Encoding:     iotextypes.Encoding_TX_CONTAINER,
		})
		require.NoError(t, err)
		require.Equal(t, uint32(iotextypes.Encoding_TX_CONTAINER), selp.Encoding())
		return selp
	}

	createStake, err := action.NewCreateStake("cand1", "100", 1, true, nil)
	require.NoError(t, err)
	userActions := []struct {
		name  string
		build func(*testing.T, uint64) *action.SealedEnvelope
	}{
		{"transfer", func(t *testing.T, gas uint64) *action.SealedEnvelope {
			return sign(t, gas, action.NewTransfer(big.NewInt(1), to, nil))
		}},
		{"execution", func(t *testing.T, gas uint64) *action.SealedEnvelope {
			return sign(t, gas, action.NewExecution(to, big.NewInt(0), []byte{1, 2, 3}))
		}},
		{"contract creation", func(t *testing.T, gas uint64) *action.SealedEnvelope {
			return sign(t, gas, action.NewExecution("", big.NewInt(0), []byte{0x00}))
		}},
		{"create stake", func(t *testing.T, gas uint64) *action.SealedEnvelope {
			return sign(t, gas, createStake)
		}},
		{"unstake", func(t *testing.T, gas uint64) *action.SealedEnvelope {
			return sign(t, gas, action.NewUnstake(1, nil))
		}},
		{"migrate stake", func(t *testing.T, gas uint64) *action.SealedEnvelope {
			return sign(t, gas, action.NewMigrateStake(1))
		}},
		{"claim reward", func(t *testing.T, gas uint64) *action.SealedEnvelope {
			return sign(t, gas, action.NewClaimFromRewardingFund(big.NewInt(1), nil, nil))
		}},
		{"deposit to rewarding fund", func(t *testing.T, gas uint64) *action.SealedEnvelope {
			return sign(t, gas, action.NewDepositToRewardingFund(big.NewInt(1), nil))
		}},
		{"tx container", container},
	}

	ctxAt := func(height uint64) context.Context {
		ctx := WithBlockCtx(context.Background(), BlockCtx{
			BlockHeight: height,
			Producer:    identityset.Address(27),
		})
		return WithFeatureCtx(genesis.WithGenesisContext(ctx, g))
	}
	for _, height := range []uint64{forkHeight - 1, forkHeight} {
		ctx := ctxAt(height)
		postFork := height >= forkHeight
		require.Equal(t, postFork, MustGetFeatureCtx(ctx).EnableOsakaEVM)
		for _, ua := range userActions {
			for _, gas := range []uint64{action.MaxTxGas, action.MaxTxGas + 1, 25_000_000} {
				selp := ua.build(t, gas)
				require.Equal(t, gas, selp.Gas())
				rejected := postFork && gas > action.MaxTxGas
				t.Run("", func(t *testing.T) {
					for name, validate := range map[string]func(context.Context, *action.SealedEnvelope) error{
						"Validate":          v.Validate,
						"ValidateWithState": v.ValidateWithState,
					} {
						err := validate(ctx, selp)
						if rejected {
							require.ErrorIs(t, err, action.ErrGasLimitTooHigh, "%s %s height %d gas %d", name, ua.name, height, gas)
						} else {
							require.NoError(t, err, "%s %s height %d gas %d", name, ua.name, height, gas)
						}
					}
				})
			}
		}
	}

	t.Run("system actions are not capped", func(t *testing.T) {
		r := require.New(t)
		ctx := ctxAt(forkHeight)
		for _, act := range []payload{
			action.NewGrantReward(action.BlockReward, forkHeight),
			action.NewPutPollResult(forkHeight, nil),
			action.NewScheduleCandidateDeactivation(identityset.Address(1)),
		} {
			elp := (&action.EnvelopeBuilder{}).SetNonce(0).SetGasLimit(1 << 40).SetAction(act).Build()
			selp, err := action.Sign(elp, identityset.PrivateKey(27))
			r.NoError(err)
			r.True(action.IsSystemAction(selp))
			r.NoError(v.Validate(ctx, selp), "%T", act)
			r.NoError(v.ValidateWithState(ctx, selp), "%T", act)
		}
	})
}
