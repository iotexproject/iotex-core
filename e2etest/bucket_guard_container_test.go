// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// guardContainerEnv is a chain with bucket 1 of owner 3 guarded by key, plus
// helpers that wrap guard actions in TX_CONTAINERs from sender 4.
type guardContainerEnv struct {
	f          *guardFixture
	owner      address.Address
	key, other *signer.PrivateKey
}

const guardContainerSender = 4

func newGuardContainerEnv(t *testing.T) *guardContainerEnv {
	r := require.New(t)
	f := newGuardFixture(t)
	const ownerID = 3
	e := &guardContainerEnv{f: f, owner: identityset.Address(ownerID), key: guardKey(0x39), other: guardKey(0x3a)}
	bc, ap := f.test.cs.Blockchain(), f.test.cs.ActionPool()
	for _, a := range []*actionWithTime{
		f.register(1, "cand1"),
		f.createStake(ownerID, "cand1", guardStakeAmount),
		f.createStake(ownerID, "cand1", guardStakeAmount),
		f.createStake(ownerID, "cand1", guardStakeAmount),
		f.signed(ownerID, f.bind(1, e.key, 1, e.owner)),
	} {
		_, rcpt, _, err := addOneTx(context.Background(), ap, bc, a)
		r.NoError(err)
		r.EqualValues(iotextypes.ReceiptStatus_Success, rcpt.Status)
	}
	return e
}

// container wraps act in a web3 transaction to the staking address with the
// given gas limit, as eth_sendRawTransaction would.
func (e *guardContainerEnv) container(t *testing.T, act *action.GuardedBucketOp, gas uint64) *action.SealedEnvelope {
	f, bc := e.f, e.f.test.cs.Blockchain()
	data := mustNoErr(act.EthData())
	staking := mustNoErr(address.FromString(address.StakingProtocolAddr))
	to := common.BytesToAddress(staking.Bytes())
	network := bc.EvmNetworkID()
	tx := types.MustSignNewTx(identityset.PrivateKey(guardContainerSender).EcdsaPrivateKey().(*ecdsa.PrivateKey),
		types.LatestSignerForChainID(new(big.Int).SetUint64(uint64(network))),
		&types.LegacyTx{Nonce: f.nonce(guardContainerSender), GasPrice: f.gasPrice, Gas: gas, To: &to, Value: new(big.Int), Data: data})
	raw := mustNoErr(tx.MarshalBinary())
	_, sig, pub, err := action.ExtractTypeSigPubkey(tx)
	require.NoError(t, err)
	return mustNoErr((&action.Deserializer{}).SetEvmNetworkID(network).ActionToSealedEnvelope(&iotextypes.Action{
		Core: &iotextypes.ActionCore{ChainID: f.chainID, Action: &iotextypes.ActionCore_TxContainer{
			TxContainer: &iotextypes.TxContainer{Raw: raw}}},
		SenderPubKey: pub.Bytes(), Signature: sig, Encoding: iotextypes.Encoding_TX_CONTAINER,
	}))
}

// TestBucketGuardInTxContainer covers guard actions sent as web3 transactions
// in a TX_CONTAINER. The pool checks the container, which neither reads the
// calldata nor prices it with G_VERIFY, so a guard action that fails its own
// stateless checks gets into the pool. It must not get into a block: had it
// been minted, a node that had not seen it in its pool would reject the block
// while one that had would accept it. Reported in review of iotex-core#5059.
// Each case runs on its own chain.
func TestBucketGuardInTxContainer(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		act  func(*guardContainerEnv) *action.GuardedBucketOp
		gas  func(*guardContainerEnv, *action.GuardedBucketOp) uint64
	}{
		{"ROTATE without newGuard, guard-signed", func(e *guardContainerEnv) *action.GuardedBucketOp {
			return e.f.guarded(e.key, e.owner, e.f.op(1, bucketguard.OpRotate, 1, 0))
		}, func(e *guardContainerEnv, _ *action.GuardedBucketOp) uint64 { return e.f.gasLimit }},
		{"ROTATE without newGuard, any signature", func(e *guardContainerEnv) *action.GuardedBucketOp {
			op := e.f.op(1, bucketguard.OpRotate, 1, 0)
			op.Sig = mustNoErr(e.other.Sign([]byte(bucketguard.CallContext), []byte("not this op")))
			return action.NewGuardedBucketOp(op)
		}, func(e *guardContainerEnv, _ *action.GuardedBucketOp) uint64 { return e.f.gasLimit }},
		{"gas limit below the guard intrinsic gas", func(e *guardContainerEnv) *action.GuardedBucketOp {
			return e.f.guarded(e.key, e.owner, e.f.op(1, bucketguard.OpUnstake, 1, 0))
		}, func(_ *guardContainerEnv, a *action.GuardedBucketOp) uint64 { return mustNoErr(a.IntrinsicGas()) - 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			e := newGuardContainerEnv(t)
			defer e.f.test.teardown()
			bc, ap := e.f.test.cs.Blockchain(), e.f.test.cs.ActionPool()
			act := tc.act(e)
			se := e.container(t, act, tc.gas(e, act))
			r.NoError(ap.Add(ctx, se), "the pool sees only the container")
			h := mustNoErr(se.Hash())
			blk, err := bc.MintNewBlock(e.f.tick())
			r.NoError(err)
			for _, a := range blk.Actions {
				r.NotEqual(h, mustNoErr(a.Hash()), "a guard action failing its stateless checks must not be minted")
			}
		})
	}

	t.Run("well-formed op, and a validator without the pool", func(t *testing.T) {
		r := require.New(t)
		e := newGuardContainerEnv(t)
		defer e.f.test.teardown()
		f, bc, ap := e.f, e.f.test.cs.Blockchain(), e.f.test.cs.ActionPool()
		// signed correctly and priced by its own gas, it goes through
		act := f.guarded(e.key, e.owner, f.op(1, bucketguard.OpUnstake, 1, 0))
		_, rcpt, _, err := addOneTx(ctx, ap, bc, &actionWithTime{e.container(t, act, mustNoErr(act.IntrinsicGas())), f.tick()})
		r.NoError(err)
		r.EqualValues(iotextypes.ReceiptStatus_Success, rcpt.Status)

		// a validator that never saw a malformed op in its pool rejects it
		vctx := mustNoErr(bc.Context(ctx))
		vctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(vctx, protocol.BlockCtx{BlockHeight: bc.TipHeight() + 1}))
		malformed := f.guarded(e.key, e.owner, f.op(1, bucketguard.OpRotate, 1, 1))
		elp := (&action.EnvelopeBuilder{}).SetNonce(f.nonce(guardContainerSender)).SetGasLimit(f.gasLimit).
			SetGasPrice(f.gasPrice).SetChainID(f.chainID).SetAction(malformed).Build()
		se := mustNoErr(action.Sign(elp, identityset.PrivateKey(guardContainerSender)))
		r.Error(protocol.NewGenericValidator(f.test.cs.StateFactory(), accountutil.AccountState).Validate(vctx, se))
	})
}
