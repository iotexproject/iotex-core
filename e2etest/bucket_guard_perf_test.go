// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"math/big"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

const (
	qsbPerfEnv       = "QSB_PERF"
	qsbPerfRoundsEnv = "QSB_PERF_ROUNDS"
)

// TestBucketGuardBlockPerf times blocks filled to the block gas limit with
// guarded ops, through the full node path: the action pool, block execution
// on the state database (state reads and writes, the staking snapshot, and
// its revert on failure), receipts, commit, and decoding the block. It
// supplies the full-handler measurements of IIP-65 §8 for G_VERIFY.
//
// Set QSB_PERF=1 to run it, and QSB_PERF_ROUNDS (default 5) for the blocks
// per scenario. On amd64, GODEBUG=cpu.sha=off runs it without the SHA
// extensions.
func TestBucketGuardBlockPerf(t *testing.T) {
	if os.Getenv(qsbPerfEnv) == "" {
		t.Skipf("set %s=1 to run the bucket guard block benchmark", qsbPerfEnv)
	}
	rounds := 5
	if v := os.Getenv(qsbPerfRoundsEnv); v != "" {
		n, err := strconv.Atoi(v)
		require.NoError(t, err)
		rounds = n
	}
	r := require.New(t)
	f := newGuardFixture(t, func(cfg *config.Config) {
		// mainnet's block gas limit since Wake
		cfg.Genesis.BlockGasLimit = 25_000_000
		cfg.Genesis.TsunamiBlockGasLimit = 25_000_000
		cfg.Genesis.WakeBlockGasLimit = 25_000_000
		// free ports, so that it can run beside a node
		cfg.API.GRPCPort = testutil.RandomPort()
		cfg.API.HTTPPort = testutil.RandomPort()
		cfg.API.WebSocketPort = testutil.RandomPort()
		cfg.Network.Port = testutil.RandomPort()
		cfg.System.HTTPStatsPort = 0
	})
	defer f.test.teardown()
	// Every measured block is full, so the base fee rises 12.5% per block;
	// pay enough to stay above it for the whole run.
	f.gasPrice = big.NewInt(unit.Qev * 1000)
	const ownerID, sponsorID = 2, 1
	owner := identityset.Address(ownerID)
	bc, ap := f.test.cs.Blockchain(), f.test.cs.ActionPool()
	g := f.test.cfg.Genesis

	add := func(se *action.SealedEnvelope) {
		ctx, err := bc.Context(context.Background())
		r.NoError(err)
		ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: bc.TipHeight() + 1}))
		r.NoError(ap.Add(ctx, se))
	}
	// signExact signs p for id with its intrinsic gas as the gas limit, so
	// that a block holds as many ops as the block gas limit allows.
	signExact := func(id int, p guardPayload) *action.SealedEnvelope {
		gas, err := p.IntrinsicGas()
		r.NoError(err)
		elp := (&action.EnvelopeBuilder{}).SetNonce(f.nonce(id)).
			SetGasLimit(gas).SetGasPrice(f.gasPrice).SetChainID(f.chainID).SetAction(p).Build()
		return mustNoErr(action.Sign(elp, identityset.PrivateKey(id)))
	}
	// now is the next block time: the wall clock, so that signing between
	// blocks does not leave the minter's deadline (block time + timeout) in
	// the past, which would stop it executing actions.
	now := func() time.Time {
		t := time.Now()
		if !t.After(f.blockTime) {
			t = f.blockTime.Add(time.Millisecond)
		}
		f.blockTime = t
		return t
	}
	mint := func() *block.Block {
		blk := mustNoErr(bc.MintNewBlock(now()))
		r.NoError(bc.CommitBlock(blk))
		ap.Reset()
		return blk
	}
	drain := func() {
		for n := 0; len(ap.PendingActionMap()) > 0; n++ {
			r.Less(n, 100, "the pool does not drain")
			blk := mint()
			for i, rcpt := range blk.Receipts {
				if _, ok := blk.Actions[i].Envelope.Action().(*action.GrantReward); ok {
					continue // the test genesis has no reward fund
				}
				r.EqualValues(iotextypes.ReceiptStatus_Success, rcpt.Status, "height %d: %T", blk.Height(), blk.Actions[i].Envelope.Action())
			}
		}
	}
	parallel := func(n int, fn func(i int)) {
		var wg sync.WaitGroup
		sem := make(chan struct{}, runtime.NumCPU())
		for i := 0; i < n; i++ {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer wg.Done()
				fn(i)
				<-sem
			}(i)
		}
		wg.Wait()
	}

	// Ops per block: the block gas limit over the gas of one op with one
	// verification (n1) and with two (n2). The margin covers the varint
	// growth of the anchor height.
	gasLimit := g.BlockGasLimitByHeight(bc.TipHeight() + 1)
	opGas := func(withNewGuard bool) uint64 {
		op := f.op(1, bucketguard.OpUnstake, 1, 0)
		op.Sig = make([]byte, guardAlg.SignatureSize())
		if withNewGuard {
			op.Op = bucketguard.OpRotate
			op.NewGuard = &bucketguard.NewGuard{AlgID: guardAlg, PublicKey: make([]byte, guardAlg.PublicKeySize()), AcceptSig: op.Sig}
		}
		return mustNoErr(action.NewGuardedBucketOp(op).IntrinsicGas()) + 1_000
	}
	n1, n2 := int(gasLimit/opGas(false)), int(gasLimit/opGas(true))
	t.Logf("qsb-perf setup block_gas_limit=%d one_verify_ops=%d two_verify_ops=%d rounds=%d cpus=%d",
		gasLimit, n1, n2, rounds, runtime.NumCPU())

	// Bucket 0 is cand1's self-stake. Buckets 1..n2 are rotated and receive
	// the failing ops; each round of UNSTAKE uses n1 fresh buckets.
	nBuckets := n2 + n1*rounds
	add(f.register(sponsorID, "cand1").act)
	drain()
	for i := 0; i < nBuckets; i++ {
		add(f.createStake(ownerID, "cand1", guardStakeAmount).act)
	}
	drain()
	for bc.TipHeight() < guardFork-1 {
		mint()
	}
	keys := [2]*signer.PrivateKey{guardKey(0xa), guardKey(0xb)}
	binds := make([]*action.BindBucketGuard, nBuckets)
	parallel(nBuckets, func(i int) { binds[i] = f.bind(uint64(i+1), keys[0], 1, owner) })
	for _, b := range binds {
		add(signExact(ownerID, b))
	}
	drain()

	type state struct{ gen, nonce uint64 }
	guards := make(map[uint64]*state, nBuckets)
	for i := 1; i <= nBuckets; i++ {
		guards[uint64(i)] = &state{gen: 1}
	}
	keyOf := func(s *state) *signer.PrivateKey { return keys[(s.gen-1)%2] }

	type sample struct {
		mint, commit, decode time.Duration
		ops                  int
		gas                  uint64
	}
	results := map[string][]sample{}
	measure := func(name string, ops []*action.GuardedBucketOp, want iotextypes.ReceiptStatus) {
		hashes := map[hash.Hash256]bool{}
		for _, op := range ops {
			se := signExact(sponsorID, op)
			hashes[mustNoErr(se.Hash())] = true
			add(se)
		}
		runtime.GC()
		start := time.Now()
		blk := mustNoErr(bc.MintNewBlock(now()))
		minted := time.Since(start)
		start = time.Now()
		r.NoError(bc.CommitBlock(blk))
		committed := time.Since(start)
		ap.Reset()
		buf := mustNoErr(blk.Serialize())
		start = time.Now()
		_, err := block.NewDeserializer(f.test.cfg.Chain.EVMNetworkID).DeserializeBlock(buf)
		decoded := time.Since(start)
		r.NoError(err)
		var n int
		var gas uint64
		for _, rcpt := range blk.Receipts {
			if hashes[rcpt.ActionHash] {
				n++
				gas += rcpt.GasConsumed
				r.EqualValues(want, rcpt.Status, name)
			}
		}
		r.Equal(len(ops), n, "%s: every op must be in the block", name)
		results[name] = append(results[name], sample{minted, committed, decoded, n, gas})
		t.Logf("qsb-perf scenario=%s height=%d ops=%d gas=%d bytes=%d mint=%v commit=%v decode=%v",
			name, blk.Height(), n, gas, len(buf), minted, committed, decoded)
	}
	sign := func(k *signer.PrivateKey, op *bucketguard.GuardedBucketOp) *action.GuardedBucketOp {
		return f.guarded(k, owner, op)
	}

	for round := 0; round < rounds; round++ {
		measure("empty", nil, iotextypes.ReceiptStatus_Success)

		// UNSTAKE: one verification each, on fresh buckets.
		ops := make([]*action.GuardedBucketOp, n1)
		parallel(n1, func(i int) {
			idx := uint64(n2 + n1*round + i + 1)
			s := guards[idx]
			ops[i] = sign(keyOf(s), f.op(idx, bucketguard.OpUnstake, s.gen, s.nonce))
		})
		measure("unstake", ops, iotextypes.ReceiptStatus_Success)

		// ROTATE: two verifications and about 32.6 KB each.
		ops = make([]*action.GuardedBucketOp, n2)
		parallel(n2, func(i int) {
			idx := uint64(i + 1)
			s := guards[idx]
			op := f.op(idx, bucketguard.OpRotate, s.gen, s.nonce)
			f.newGuard(op, owner, keys[s.gen%2], owner)
			ops[i] = sign(keyOf(s), op)
		})
		measure("rotate", ops, iotextypes.ReceiptStatus_Success)
		for i := 1; i <= n2; i++ {
			guards[uint64(i)].gen++
			guards[uint64(i)].nonce++
		}

		// Failing ops are charged and included: a CALL signature for
		// another message (one verification), and a valid CALL with an
		// ACCEPT for another CALL (two verifications).
		ops = make([]*action.GuardedBucketOp, n1)
		parallel(n1, func(i int) {
			idx := uint64(i%n2 + 1)
			s := guards[idx]
			op := f.op(idx, bucketguard.OpUnstake, s.gen, s.nonce)
			op.Sig = mustNoErr(keyOf(s).Sign([]byte(bucketguard.CallContext), []byte("another message")))
			ops[i] = action.NewGuardedBucketOp(op)
		})
		measure("bad-call", ops, iotextypes.ReceiptStatus_ErrGuardSignature)
		ops = make([]*action.GuardedBucketOp, n2)
		parallel(n2, func(i int) {
			idx := uint64(i + 1)
			s := guards[idx]
			op := f.op(idx, bucketguard.OpRotate, s.gen, s.nonce)
			f.newGuard(op, owner, keys[s.gen%2], identityset.Address(9)) // ACCEPT for another owner
			ops[i] = sign(keyOf(s), op)
		})
		measure("bad-accept", ops, iotextypes.ReceiptStatus_ErrGuardSignature)
	}

	median := func(d []time.Duration) time.Duration {
		sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
		return d[len(d)/2]
	}
	medians := func(name string) (mint, commit, decode time.Duration) {
		var m, c, d []time.Duration
		for _, s := range results[name] {
			m, c, d = append(m, s.mint), append(c, s.commit), append(d, s.decode)
		}
		return median(m), median(c), median(d)
	}
	// Per op: the block's execution time (MintNewBlock) above an empty
	// block's, over its ops. Commit writes the database and is reported
	// separately.
	emptyMint, emptyCommit, _ := medians("empty")
	for _, name := range []string{"unstake", "rotate", "bad-call", "bad-accept"} {
		mint, commit, decode := medians(name)
		s := results[name][0]
		perOp := (mint - emptyMint) / time.Duration(s.ops)
		t.Logf("qsb-perf summary scenario=%s ops=%d gas_per_op=%d median_mint=%v empty_mint=%v exec_per_op=%v median_commit=%v empty_commit=%v median_decode=%v",
			name, s.ops, s.gas/uint64(s.ops), mint, emptyMint, perOp, commit, emptyCommit, decode)
	}
}
