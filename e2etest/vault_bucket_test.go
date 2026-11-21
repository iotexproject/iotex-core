// Copyright (c) 2026 IoTeX Foundation
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha512"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/api"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/server/itx"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestVaultBucketDogfood submits ECDSA transactions, real SLH-DSA signatures,
// mines and commits blocks, and reads balances from the node state factory.
func TestVaultBucketDogfood(t *testing.T) {
	f := newVaultChainFixture(t)
	apiServer, err := f.test.cs.NewAPIServer(api.DefaultConfig, false)
	require.NoError(t, err)
	core := apiServer.CoreService()
	rpcClient, err := rpc.DialOptions(context.Background(), "http://vault.test", rpc.WithHTTPClient(&http.Client{Transport: guardRPCTransport{api.NewWeb3Handler(core, "", 100)}}))
	require.NoError(t, err)
	t.Cleanup(rpcClient.Close)
	eth := ethclient.NewClient(rpcClient)
	key, newKey := guardKey(0x51), guardKey(0x52)
	const ownerID = 3
	owner := identityset.Address(ownerID)
	// Reach the configured fork through ordinary user transactions.
	f.run([]*testcase{{name: "bootstrap", preActs: []*actionWithTime{f.register(1, "cand1"), f.createStake(ownerID, "cand1", guardStakeAmount), f.createStake(ownerID, "cand1", guardStakeAmount)}, act: f.createStake(ownerID, "cand1", guardStakeAmount), expect: []actionExpect{successExpect}}})
	send := func(id int, m *bucketguard.VaultAction, success bool) *action.Receipt {
		a, err := action.NewVaultAction(m)
		require.NoError(t, err)

		tx := vaultWeb3Tx(t, f, id, a)
		to := mustNoErr(a.EthTo())
		gas, estimateErr := eth.EstimateGas(context.Background(), ethereum.CallMsg{From: common.BytesToAddress(identityset.Address(id).Bytes()), To: to, GasPrice: f.gasPrice, Data: mustNoErr(a.EthData())})
		var r *action.Receipt
		if success {
			require.NoError(t, estimateErr)
			require.Equal(t, mustNoErr(a.IntrinsicGas()), gas)
			_, r, _, err = addOneTx(context.Background(), f.test.cs.ActionPool(), f.test.cs.Blockchain(), tx)
		} else {
			require.Error(t, estimateErr)
			ctx, e := f.test.cs.Blockchain().Context(context.Background())
			require.NoError(t, e)
			require.Error(t, f.test.cs.ActionPool().Add(ctx, tx.act))
			require.Zero(t, f.test.cs.ActionPool().GetSize())
			_, r, _, err = f.packDirect(t, tx)
		}
		require.NoError(t, err)
		require.NotNil(t, r)
		require.Zero(t, f.test.cs.ActionPool().GetGasSize(), "container removal refunds its admission gas")
		if success {
			require.EqualValues(t, iotextypes.ReceiptStatus_Success, r.Status)
			require.NotEmpty(t, r.Logs())
		} else {
			require.NotEqualValues(t, iotextypes.ReceiptStatus_Success, r.Status)
			require.NotEmpty(t, r.ExecutionRevertMsg())
		}
		pool := bucketguard.VaultPool()
		poolAddr, err := address.FromBytes(pool[:])
		require.NoError(t, err)
		balance, err := accountutil.LoadAccount(f.test.cs.StateFactory(), poolAddr)
		require.NoError(t, err)
		v, e := staking.ReadVaultBucket(f.test.cs.StateFactory(), 0)
		amount := new(big.Int)
		if e == nil {
			amount.SetBytes(v.Amount)
		}
		require.Equal(t, amount, balance.Balance, "pool equals live liquid principal")
		if e == nil {
			getters, err := abi.JSON(strings.NewReader(bucketguard.VaultGettersABI))
			require.NoError(t, err)
			getter := getters.Methods["vaultBucket"]
			arg, err := getter.Inputs.Pack(uint64(0))
			require.NoError(t, err)
			ctx, err := f.test.cs.Blockchain().Context(context.Background())
			require.NoError(t, err)
			ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: f.test.cs.Blockchain().TipHeight()}))
			result, _, err := staking.FindProtocol(f.test.cs.Registry()).ReadState(ctx, f.test.cs.StateFactory(), []byte("vaultBucket"), arg)
			require.NoError(t, err)
			vals, err := getter.Outputs.Unpack(result)
			require.NoError(t, err)
			require.Equal(t, common.BytesToAddress(v.Owner), vals[0])
			require.Equal(t, amount, vals[1])
		}
		return r
	}
	sf := f.test.cs.StateFactory()
	read := func() (*staking.VaultBucketRecord, *staking.BucketGuardV2Record) {
		v, err := staking.ReadVaultBucket(sf, 0)
		require.NoError(t, err)
		g, err := staking.ReadVaultGuard(sf, 0)
		require.NoError(t, err)
		return v, g
	}
	create := &bucketguard.VaultAction{Method: "vaultCreate", Amount: unit.ConvertIotxToRau(500), OwnerTxNonce: f.test.nonceMgr[owner.String()], Guard: bucketguard.NewGuard{AlgID: guardAlg, PublicKey: key.PublicKey().Bytes()}}
	create.Guard.AcceptSig = mustNoErr(key.Sign([]byte(bucketguard.VaultAcceptContext), bucketguard.EncodeVaultAccept(f.domain, 0, create.OwnerTxNonce, 1, owner.Bytes(), bucketguard.UnbindDelayBlocks, guardAlg, create.Guard.PublicKey, [64]byte{})))
	below := *create
	below.Amount = unit.ConvertIotxToRau(99)
	send(ownerID, &below, false)
	create.OwnerTxNonce = f.test.nonceMgr[owner.String()]
	create.Guard.AcceptSig = mustNoErr(key.Sign([]byte(bucketguard.VaultAcceptContext), bucketguard.EncodeVaultAccept(f.domain, 0, create.OwnerTxNonce, 1, owner.Bytes(), bucketguard.UnbindDelayBlocks, guardAlg, create.Guard.PublicKey, [64]byte{})))
	start := f.balance(ownerID)
	send(ownerID, create, true)
	v, g := read()
	require.Equal(t, create.Amount.Bytes(), v.Amount)
	require.EqualValues(t, 1, g.Generation)
	require.True(t, new(big.Int).Sub(start, f.balance(ownerID)).Cmp(create.Amount) > 0, "owner paid principal and gas")
	send(ownerID, &bucketguard.VaultAction{Method: "vaultAddDeposit", Amount: unit.ConvertIotxToRau(100)}, true)
	signed := func(method string, op uint32, k *signer.PrivateKey, amount *big.Int, recipient, newOwner []byte) *bucketguard.VaultAction {
		v, g := read()
		h := f.test.cs.Blockchain().TipHeight()
		c := bucketguard.VaultCall{VaultIndex: 0, Generation: g.Generation, GuardNonce: g.GuardNonce, Deadline: h + 100, AnchorHeight: h, AnchorHash: f.blockHash(h), Op: op, Amount: amount, Recipient: recipient, NewOwner: newOwner}
		m := &bucketguard.VaultAction{Method: method}
		if op == bucketguard.VaultOpRotateGuard {
			m.Guard = bucketguard.NewGuard{AlgID: guardAlg, PublicKey: newKey.PublicKey().Bytes()}
			c.NewGuard = &m.Guard
		}
		msg := bucketguard.EncodeVaultCall(f.domain, v.Owner, g.KeyID(), &c)
		if c.NewGuard != nil {
			m.Guard.AcceptSig = mustNoErr(newKey.Sign([]byte(bucketguard.VaultAcceptContext), bucketguard.EncodeVaultAccept(f.domain, 0, 0, g.Generation+1, v.Owner, g.UnbindDelay, guardAlg, m.Guard.PublicKey, sha512.Sum512(msg))))
		}
		c.Sig = mustNoErr(k.Sign([]byte(bucketguard.VaultCallContext), msg))
		require.Len(t, c.Sig, 16224)
		m.Call = c
		return m
	}
	recipient := identityset.Address(5)
	poolAddress := bucketguard.VaultPool()
	invalid, err := action.NewVaultAction(signed("vaultWithdraw", bucketguard.VaultOpWithdraw, key, unit.ConvertIotxToRau(100), poolAddress[:], nil))
	require.NoError(t, err)
	require.Error(t, invalid.SanityCheck(), "pool cannot be a withdrawal recipient")
	before := f.balance(5)
	send(ownerID, signed("vaultWithdraw", bucketguard.VaultOpWithdraw, key, unit.ConvertIotxToRau(100), recipient.Bytes(), nil), true)
	require.Equal(t, unit.ConvertIotxToRau(100).String(), new(big.Int).Sub(f.balance(5), before).String(), "T+0 payout")
	// A guard signature alone cannot withdraw from an unrelated outer sender.
	before = f.balance(4)
	v, g = read()
	bad := signed("vaultWithdraw", bucketguard.VaultOpWithdraw, key, unit.ConvertIotxToRau(100), recipient.Bytes(), nil)
	require.Len(t, bad.Call.Sig, 16224)
	bad.Call.Sig[0] ^= 1
	send(ownerID, bad, false)
	send(4, signed("vaultWithdraw", bucketguard.VaultOpWithdraw, key, unit.ConvertIotxToRau(100), recipient.Bytes(), nil), false)
	v2, g2 := read()
	require.Equal(t, v, v2)
	require.Equal(t, g, g2)
	require.True(t, f.balance(4).Cmp(before) < 0, "failed action still pays gas")
	send(ownerID, signed("vaultWithdraw", bucketguard.VaultOpWithdraw, key, unit.ConvertIotxToRau(450), recipient.Bytes(), nil), false)
	// Deploy a minimal EVM contract to test the actual account-code check.
	deploy := f.signed(ownerID, action.NewExecution("", new(big.Int), common.FromHex("0x6001600c60003960016000f300")))
	_, deployed, _, deployErr := addOneTx(context.Background(), f.test.cs.ActionPool(), f.test.cs.Blockchain(), deploy)
	require.NoError(t, deployErr)
	require.EqualValues(t, iotextypes.ReceiptStatus_Success, deployed.Status)
	contract := mustNoErr(address.FromString(deployed.ContractAddress))
	send(ownerID, signed("vaultRotateOwner", bucketguard.VaultOpRotateOwner, key, new(big.Int), nil, contract.Bytes()), false)
	send(ownerID, signed("vaultRotateOwner", bucketguard.VaultOpRotateOwner, key, new(big.Int), nil, identityset.Address(6).Bytes()), true)
	badRotate := signed("vaultRotateGuard", bucketguard.VaultOpRotateGuard, key, new(big.Int), nil, nil)
	require.Equal(t, 32448, len(badRotate.Call.Sig)+len(badRotate.Guard.AcceptSig))
	badRotate.Guard.AcceptSig[0] ^= 1
	send(6, badRotate, false)
	send(6, signed("vaultRotateGuard", bucketguard.VaultOpRotateGuard, key, new(big.Int), nil, nil), true)
	oldPresign := signed("vaultWithdraw", bucketguard.VaultOpWithdraw, newKey, unit.ConvertIotxToRau(100), recipient.Bytes(), nil)
	send(4, signed("vaultRevokeSignatures", bucketguard.VaultOpRevokeSignatures, newKey, new(big.Int), nil, nil), true)
	send(6, oldPresign, false)
	send(6, &bucketguard.VaultAction{Method: "vaultInitiateExit"}, true)
	_, g = read()
	expiry := g.Pending.ExpiresAt
	send(6, &bucketguard.VaultAction{Method: "vaultAddDeposit", Amount: unit.ConvertIotxToRau(100)}, false)
	send(6, signed("vaultRotateOwner", bucketguard.VaultOpRotateOwner, newKey, new(big.Int), nil, owner.Bytes()), false)
	send(6, signed("vaultWithdraw", bucketguard.VaultOpWithdraw, newKey, unit.ConvertIotxToRau(100), recipient.Bytes(), nil), true)
	_, g = read()
	require.Equal(t, expiry, g.Pending.ExpiresAt)
	send(6, &bucketguard.VaultAction{Method: "vaultCancelExit"}, true)
	send(6, &bucketguard.VaultAction{Method: "vaultInitiateExit"}, true)
	send(6, &bucketguard.VaultAction{Method: "vaultFinalizeExit"}, false)
	// A dual-authorized full withdrawal also leaves a permanent tombstone.
	v, _ = read()
	send(6, signed("vaultWithdraw", bucketguard.VaultOpWithdraw, newKey, new(big.Int).SetBytes(v.Amount), recipient.Bytes(), nil), true)
	_, err = staking.ReadVaultBucket(sf, 0)
	require.Error(t, err)
	tomb, err := staking.ReadVaultGuard(sf, 0)
	require.NoError(t, err)
	require.Empty(t, tomb.PublicKey)
	send(6, &bucketguard.VaultAction{Method: "vaultFinalizeExit"}, false)
}

// newVaultChainFixture starts the real node chain service, state factory,
// action pool and block DAO in process. RPC and P2P listeners are unnecessary
// for these transaction/commit tests and cannot open inside a restricted runner.
func newVaultChainFixture(t *testing.T) *guardFixture {
	cfg := initCfg(require.New(t))
	cfg.Genesis.ToBeEnabledBlockHeight = guardFork
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	svr, err := itx.NewServer(cfg)
	require.NoError(t, err)
	ctx := genesis.WithGenesisContext(context.Background(), cfg.Genesis)
	ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{}))
	cs := svr.ChainService(cfg.Chain.ID)
	require.NoError(t, cs.Start(ctx))
	t.Cleanup(func() { require.NoError(t, cs.Stop(ctx)); clearDBPaths(&cfg) })
	e := &e2etest{cfg: cfg, svr: svr, cs: cs, t: t, nonceMgr: make(accountNonceManager)}
	now := time.Now()
	return &guardFixture{test: e, chainID: cfg.Chain.ID, domain: bucketguard.NewDomain(cfg.Chain.ID, cfg.Genesis.Hash()), gasLimit: 4_000_000, gasPrice: big.NewInt(unit.Qev*2 + 1), start: now, blockTime: now, pending: map[*actionWithTime]pendingAct{}}
}

func TestVaultBucketClassBRecovery(t *testing.T) {
	f := newVaultChainFixture(t)
	apiServer, err := f.test.cs.NewAPIServer(api.DefaultConfig, false)
	require.NoError(t, err)
	core := apiServer.CoreService()
	const id = 3
	owner := identityset.Address(id)
	key := guardKey(0x53)
	f.run([]*testcase{{name: "bootstrap", preActs: []*actionWithTime{f.register(1, "cand1"), f.createStake(id, "cand1", guardStakeAmount), f.createStake(id, "cand1", guardStakeAmount)}, act: f.createStake(id, "cand1", guardStakeAmount), expect: []actionExpect{successExpect}}})
	send := func(m *bucketguard.VaultAction, ok bool) {
		a, err := action.NewVaultAction(m)
		require.NoError(t, err)

		tx := vaultWeb3Tx(t, f, id, a)
		elp := (&action.EnvelopeBuilder{}).SetAction(a).SetGasLimit(f.gasLimit).SetGasPrice(f.gasPrice).SetNonce(tx.act.Nonce()).Build()
		gas, estimateErr := core.EstimateBucketGuardGas(context.Background(), elp, owner)
		if ok {
			require.NoError(t, estimateErr)
			require.Equal(t, mustNoErr(a.IntrinsicGas()), gas)
		} else {
			require.Error(t, estimateErr)
		}
		var r *action.Receipt
		if ok {
			_, r, _, err = addOneTx(context.Background(), f.test.cs.ActionPool(), f.test.cs.Blockchain(), tx)
		} else {
			require.Error(t, f.test.cs.ActionPool().Add(context.Background(), tx.act))
			_, r, _, err = f.packDirect(t, tx)
		}
		require.NoError(t, err)
		require.NotNil(t, r)
		require.Zero(t, f.test.cs.ActionPool().GetGasSize(), "container removal refunds its admission gas")
		require.Equal(t, ok, r.Status == uint64(iotextypes.ReceiptStatus_Success))
	}
	const index = 1
	bind := &bucketguard.VaultAction{Method: "bindBucketGuardV2", Index: index, UnbindDelay: bucketguard.UnbindDelayBlocks, Guard: bucketguard.NewGuard{AlgID: guardAlg, PublicKey: key.PublicKey().Bytes()}}
	bind.Guard.AcceptSig = mustNoErr(key.Sign([]byte(bucketguard.AcceptContext), bucketguard.EncodeAcceptBV2(f.domain, index, 1, owner.Bytes(), bind.UnbindDelay, guardAlg, bind.Guard.PublicKey, [64]byte{})))
	send(bind, true)
	send(&bucketguard.VaultAction{Method: "guardUnbindInitiate", Index: index}, true)
	sf := f.test.cs.StateFactory()
	g, err := staking.ReadStakingGuardV2(sf, index)
	require.NoError(t, err)
	require.NotNil(t, g.Pending)
	require.Zero(t, g.GuardNonce)
	// Run the actual IIP-59 deposit entry on an uncommitted real working set.
	ctx, err := f.test.cs.Blockchain().Context(context.Background())
	require.NoError(t, err)
	ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: f.test.cs.Blockchain().TipHeight() + 1}))
	ws, err := sf.WorkingSet(ctx)
	require.NoError(t, err)
	sp := staking.FindProtocol(f.test.cs.Registry())
	csr, err := staking.NewCandidateStateManagerWithContext(ctx, ws)
	require.NoError(t, err)
	b, err := csr.NativeBucket(index)
	require.NoError(t, err)
	principal := new(big.Int).Set(b.StakedAmount)
	require.ErrorIs(t, sp.AddDepositForCompound(ctx, ws, owner, index, big.NewInt(123), staking.FrozenSelfStake{}), staking.ErrCompoundSkippedPendingUnbind)
	b, err = csr.NativeBucket(index)
	require.NoError(t, err)
	require.Equal(t, principal, b.StakedAmount)
	after, err := staking.ReadStakingGuardV2(ws, index)
	require.NoError(t, err)
	require.Equal(t, g, after)
	ws.Close()
	h := f.test.cs.Blockchain().TipHeight()
	cancel := &bucketguard.VaultAction{Method: "guardUnbindCancel", Index: index, RequestID: g.Pending.RequestID, Call: bucketguard.VaultCall{Generation: g.Generation, Deadline: h + 100, AnchorHeight: h, AnchorHash: f.blockHash(h)}}
	cancel.Call.Sig = mustNoErr(key.Sign([]byte(bucketguard.GuardCancelContext), bucketguard.EncodeGuardCancel(f.domain, &bucketguard.GuardCancel{BucketIndex: index, Generation: g.Generation, RequestID: cancel.RequestID, Deadline: cancel.Call.Deadline, AnchorHeight: h, AnchorHash: cancel.Call.AnchorHash})))
	send(cancel, true)
	send(cancel, true)
	g, err = staking.ReadStakingGuardV2(sf, index)
	require.NoError(t, err)
	require.Nil(t, g.Pending)
	require.Zero(t, g.GuardNonce)
	require.NotZero(t, g.CancelCooldown)
	send(&bucketguard.VaultAction{Method: "guardUnbindInitiate", Index: index}, false)
}

// vaultWeb3Tx follows eth_sendRawTransaction's TX_CONTAINER path, including
// real EIP-155 ECDSA signing and ABI unfolding at the staking address.
func vaultWeb3Tx(t *testing.T, f *guardFixture, id int, a *action.VaultAction) *actionWithTime {
	data, err := a.EthData()
	require.NoError(t, err)
	to, err := a.EthTo()
	require.NoError(t, err)
	network := f.test.cs.Blockchain().EvmNetworkID()
	tx := types.MustSignNewTx(identityset.PrivateKey(id).EcdsaPrivateKey().(*ecdsa.PrivateKey), types.LatestSignerForChainID(new(big.Int).SetUint64(uint64(network))), &types.LegacyTx{Nonce: f.nonce(id), GasPrice: f.gasPrice, Gas: f.gasLimit, To: to, Value: new(big.Int), Data: data})
	raw, err := tx.MarshalBinary()
	require.NoError(t, err)
	_, sig, pub, err := action.ExtractTypeSigPubkey(tx)
	require.NoError(t, err)
	se, err := (&action.Deserializer{}).SetEvmNetworkID(network).ActionToSealedEnvelope(&iotextypes.Action{Core: &iotextypes.ActionCore{ChainID: f.chainID, Action: &iotextypes.ActionCore_TxContainer{TxContainer: &iotextypes.TxContainer{Raw: raw}}}, SenderPubKey: pub.Bytes(), Signature: sig, Encoding: iotextypes.Encoding_TX_CONTAINER})
	require.NoError(t, err)
	return &actionWithTime{se, f.tick()}
}
