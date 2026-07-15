// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

//go:build iip59bench
// +build iip59bench

package execution_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// AutoDeposit storage layout, verified empirically against the mainnet
// runtime bytecode fixture (e2etest/autodeposit_bytecode). Solidity 0.4.24
// with `is Pausable, Ownable`; the mainnet-deployed contract's actual
// layout is:
//
//	slot 0 : Pausable._paused (bool)  — always 0 on our test deploy
//	                                    since we install runtime bytecode
//	                                    and skip the constructor
//	slot 1 : buckets      (mapping address => int256)
//	slot 2 : registrants  (mapping address => bool)
//
// Note: this does NOT match the naive "parent state comes first" reading
// of the source (Pausable._paused + Ownable.owner would push our maps to
// slots 2/3). Whatever the compiler/linker actually produced for the
// mainnet contract lands buckets at slot 1 — probe voter2's register(2)
// tx to disambiguate: the slot that reads 2 is buckets, the slot that
// reads 1 (bool true) is registrants. Layout is asserted at bench setup.
//
// For a Solidity mapping at slot p, storage[k] lives at
// keccak256(pad32(k) || pad32(p)). Voter addresses are 20 bytes, left-padded
// with zeros to 32.
const (
	autoDepositSlotBuckets     = uint8(1)
	autoDepositSlotRegistrants = uint8(2)
)

// mappingSlotKey returns the storage key for slot `mappingSlot`'s value
// under address key `addr`, matching Solidity's mapping storage rule:
// keccak256(pad32(k) || pad32(p)).
func mappingSlotKey(addr common.Address, mappingSlot uint8) []byte {
	buf := make([]byte, 64)
	copy(buf[12:32], addr.Bytes()) // pad20→32
	buf[63] = mappingSlot
	h := crypto.Keccak256(buf)
	return h
}

// autoDepositABIJSON is a minimal ABI covering just the two functions the
// bench needs: register(int256) for seeding and bucket(address) for the hot
// loop. Sourced from
// iotexproject/iotex-hermes/smartcontracts/contracts/AutoDepositRegister.sol
// so we can call the real setter surface against mainnet runtime bytecode.
const autoDepositBenchABIJSON = `[
    {"inputs":[{"internalType":"int256","name":"bucketId","type":"int256"}],"name":"register","outputs":[],"stateMutability":"nonpayable","type":"function"},
    {"inputs":[{"internalType":"address","name":"owner","type":"address"}],"name":"bucket","outputs":[{"internalType":"int256","name":"","type":"int256"}],"stateMutability":"view","type":"function"}
]`

// autoDepositBatchABIJSON matches AutoDepositBatch.sol
// (e2etest/autodeposit_batch.sol) — a locally-compiled wrapper that exposes
// `buckets(address[]) → int256[]` over the immutable mainnet AutoDeposit
// contract set at construction. Used by
// BenchmarkAutoDeposit_bucket_WrapperContract to measure mitigation (1a).
const autoDepositBatchABIJSON = `[
    {"inputs":[{"internalType":"address","name":"_target","type":"address"}],"stateMutability":"nonpayable","type":"constructor"},
    {"inputs":[{"internalType":"address[]","name":"owners","type":"address[]"}],"name":"buckets","outputs":[{"internalType":"int256[]","name":"","type":"int256[]"}],"stateMutability":"view","type":"function"}
]`

// loadRuntimeHex reads a hex-encoded runtime bytecode fixture from e2etest/.
// The fixtures are produced by scripts/fetch-mainnet-bytecode.sh and pinned in
// tree so the bench is offline-reproducible.
func loadRuntimeHex(tb testing.TB, name string) []byte {
	tb.Helper()
	path := "../../../e2etest/" + name
	raw, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("read fixture %s: %v", path, err)
	}
	b, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		tb.Fatalf("decode fixture %s: %v", path, err)
	}
	return b
}

// wrapRuntimeInit prepends a 12-byte deployer preamble that CODECOPYs the
// runtime bytecode into memory and RETURNs it, so runtime code obtained via
// eth_getCode (which lacks the constructor prefix) can be installed by a
// standard contract-creation transaction.
//
// Layout (offsets in the emitted code):
//
//	 0-2  61 <LEN_HI> <LEN_LO>   PUSH2 length
//	 3    80                     DUP1  (keep length for RETURN)
//	 4-5  60 0c                  PUSH1 12 (runtime lives at code offset 12)
//	 6-7  60 00                  PUSH1 0  (dest offset in memory)
//	 8    39                     CODECOPY
//	 9-10 60 00                  PUSH1 0  (return offset in memory)
//	 11   f3                     RETURN
//	 12+  <runtime bytecode>
//
// Runtime must be strictly less than 2^16 bytes (both fixtures are well under
// this bound).
func wrapRuntimeInit(runtime []byte) []byte {
	if len(runtime) >= 1<<16 {
		panic("runtime bytecode too long for PUSH2 length encoding")
	}
	l := uint16(len(runtime))
	prefix := []byte{
		0x61, byte(l >> 8), byte(l), // PUSH2 length
		0x80,       // DUP1
		0x60, 0x0c, // PUSH1 12 (runtime source offset)
		0x60, 0x00, // PUSH1 0  (mem dest)
		0x39,       // CODECOPY
		0x60, 0x00, // PUSH1 0  (mem return offset)
		0xf3, // RETURN
	}
	out := make([]byte, 0, len(prefix)+len(runtime))
	out = append(out, prefix...)
	out = append(out, runtime...)
	return out
}

// BenchmarkAutoDeposit_bucket measures per-call cost of AutoDeposit.bucket(addr)
// executed via evm.SimulateExecution — the same path used inside
// GrantEpochReward by autoDepositContractReader
// (action/protocol/rewarding/voter_reward.go). Deploys the mainnet-runtime
// AutoDeposit contract, seeds N registered voters via real register() txs, and
// hot-loops SimulateExecution calls against a rotating registered voter so the
// storage lookup always hits the populated branch — the production hot path.
//
// Invocation:
//
//	go test -tags=iip59bench -bench=BenchmarkAutoDeposit_bucket \
//	  -benchmem -count=3 -run=^$ ./action/protocol/execution/
//
// Extrapolate the reported ns/op to 107,200 (mainnet epoch worst case) to
// estimate epoch-grant contribution against the 5s block budget.
func BenchmarkAutoDeposit_bucket(b *testing.B) {
	r := require.New(b)
	ctx := context.Background()

	runtime := loadRuntimeHex(b, "autodeposit_bytecode")
	initBytecode := wrapRuntimeInit(runtime)
	autoDepositABI, err := abi.JSON(strings.NewReader(autoDepositBenchABIJSON))
	r.NoError(err)

	// 30 distinct registered voters: enough to exercise the populated
	// storage path (registrants[owner]=true; buckets[owner]=id) without
	// pulling in Address2 keys we would have to fund. Trie depth for a
	// 30-entry mapping vs a 100k-entry mapping only differs by ~10 nodes,
	// well under 1μs at the SLOAD level — the per-call cost this bench
	// reports is representative of production scale.
	const numVoters = 30

	balances := make([]ExpectedBalance, 0, numVoters+1)
	// Deployer.
	balances = append(balances, ExpectedBalance{
		Account:    identityset.Address(0).String(),
		RawBalance: "1000000000000000000000000000",
	})
	for i := 1; i <= numVoters; i++ {
		balances = append(balances, ExpectedBalance{
			Account:    identityset.Address(i).String(),
			RawBalance: "1000000000000000000000000000",
		})
	}

	sct := SmartContractTest{
		InitBalances: balances,
		Deployments: []ExecutionConfig{
			{
				ContractIndex: 0,
				RawPrivateKey: identityset.PrivateKey(0).HexString(),
				RawByteCode:   hex.EncodeToString(initBytecode),
				RawAmount:     "0",
				RawGasLimit:   10_000_000,
				RawGasPrice:   "0",
			},
		},
	}

	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg.Genesis.NumSubEpochs = 1
	cfg.Chain.ProducerPrivKey = identityset.PrivateKey(28).HexString()

	bc, sf, dao, ap := sct.prepareBlockchain(ctx, cfg, r)
	defer func() { r.NoError(bc.Stop(ctx)) }()

	contractAddrs := sct.deployContracts(bc, sf, dao, ap, r)
	r.Equal(1, len(contractAddrs))
	contractAddr := contractAddrs[0]

	// Seed: each voter registers with a distinct bucket id.
	regCfgs := make([]*ExecutionConfig, 0, numVoters)
	sameContractAddrs := make([]string, 0, numVoters)
	for i := 1; i <= numVoters; i++ {
		callData, err := autoDepositABI.Pack("register", big.NewInt(int64(i)))
		r.NoError(err)
		regCfgs = append(regCfgs, &ExecutionConfig{
			RawPrivateKey: identityset.PrivateKey(i).HexString(),
			RawByteCode:   hex.EncodeToString(callData),
			RawAmount:     "0",
			RawGasLimit:   500_000,
			RawGasPrice:   "0",
			Comment:       fmt.Sprintf("register voter %d", i),
		})
		sameContractAddrs = append(sameContractAddrs, contractAddr)
	}
	receipts, _, err := sct.runExecutions(bc, sf, dao, ap, regCfgs, sameContractAddrs)
	r.NoError(err)
	r.Equal(numVoters, len(receipts))
	for i, receipt := range receipts {
		r.Equalf(uint64(iotextypes.ReceiptStatus_Success), receipt.Status,
			"register tx %d failed: %s", i, receipt.ExecutionRevertMsg())
	}

	// Pre-pack calldata so the bench loop only measures the SimulateExecution
	// path (not ABI encoding overhead, which the production reader also
	// amortises outside the tight loop — the bridge packs once per delegate
	// snapshot, not per voter).
	voterCallData := make([][]byte, numVoters)
	for i := 0; i < numVoters; i++ {
		addr := common.BytesToAddress(identityset.Address(i + 1).Bytes())
		cd, err := autoDepositABI.Pack("bucket", addr)
		r.NoError(err)
		voterCallData[i] = cd
	}

	// Production hot loop shape: build ctx + working set + zero-address
	// caller ONCE, then dispatch through evm.SimulateExecution per call.
	// This mirrors autoDepositContractReader in
	// action/protocol/rewarding/voter_reward.go which reuses the same sm
	// across every voter in an epoch drain.
	bcCtx, err := bc.Context(context.Background())
	r.NoError(err)
	bcCtx = evm.WithHelperCtx(bcCtx, evm.HelperContext{
		GetBlockHash:   dao.GetBlockHash,
		GetBlockTime:   getBlockTimeForTest,
		DepositGasFunc: rewarding.DepositGas,
	})
	ws, err := sf.WorkingSet(bcCtx)
	r.NoError(err)
	zeroAddr, err := address.FromString(address.ZeroAddress)
	r.NoError(err)

	// Sanity check — first call must return a positive bucket id so the
	// bench is exercising the populated storage branch (registrants=true;
	// buckets=id), not the unregistered fast return (-1). If this fails,
	// register() didn't stick and every subsequent iteration is measuring
	// the wrong path.
	contractIoAddr, err := address.FromString(contractAddr)
	r.NoError(err)
	{
		ex := action.NewExecution(contractAddr, big.NewInt(0), voterCallData[0])
		elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(10_000_000).SetAction(ex).Build()
		ret, _, err := evm.SimulateExecution(bcCtx, ws, zeroAddr, elp)
		r.NoError(err)
		r.Equal(32, len(ret), "bucket() must return 32-byte int256")
		got := new(big.Int).SetBytes(ret)
		r.Equalf(int64(1), got.Int64(), "voter 1 bucket id = %s, want 1 (register() didn't persist?)", got.String())

		// Validate direct-storage-read hypothesis against the EVM result.
		// If these ever diverge, the storage layout constants above are
		// wrong for the fixture bytecode and the DirectRead bench would
		// silently measure the wrong path. Voter 2 registered with
		// bucketId=2 so we can disambiguate buckets vs registrants
		// (both would be 1 for voter 1 alone).
		voter1Evm := common.BytesToAddress(identityset.Address(1).Bytes())
		voter2Evm := common.BytesToAddress(identityset.Address(2).Bytes())

		reg1, err := evm.ReadContractStorage(bcCtx, ws, contractIoAddr,
			mappingSlotKey(voter1Evm, autoDepositSlotRegistrants))
		r.NoError(err)
		r.Equalf(byte(1), reg1[31],
			"registrants[voter1] direct read = %x, want ...01 (slot layout wrong?)", reg1)

		buck1, err := evm.ReadContractStorage(bcCtx, ws, contractIoAddr,
			mappingSlotKey(voter1Evm, autoDepositSlotBuckets))
		r.NoError(err)
		r.Equalf(int64(1), new(big.Int).SetBytes(buck1).Int64(),
			"buckets[voter1] direct read = %x, want 1", buck1)

		buck2, err := evm.ReadContractStorage(bcCtx, ws, contractIoAddr,
			mappingSlotKey(voter2Evm, autoDepositSlotBuckets))
		r.NoError(err)
		r.Equalf(int64(2), new(big.Int).SetBytes(buck2).Int64(),
			"buckets[voter2] direct read = %x, want 2 (disambiguation failed)", buck2)

		// Spot-check unregistered voter reads zero on both slots so the
		// production-side reader logic (return -1 when registrants=0)
		// remains sound under direct-read.
		strangerEvm := common.BytesToAddress(identityset.Address(numVoters + 1).Bytes())
		strangerReg, err := evm.ReadContractStorage(bcCtx, ws, contractIoAddr,
			mappingSlotKey(strangerEvm, autoDepositSlotRegistrants))
		r.NoError(err)
		r.Equalf(0, new(big.Int).SetBytes(strangerReg).Sign(),
			"unregistered voter must have zero registrants slot, got %x", strangerReg)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ex := action.NewExecution(contractAddr, big.NewInt(0), voterCallData[i%numVoters])
		elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(10_000_000).SetAction(ex).Build()
		_, _, err := evm.SimulateExecution(bcCtx, ws, zeroAddr, elp)
		if err != nil {
			b.Fatalf("SimulateExecution: %v", err)
		}
	}
	b.StopTimer()
}

// BenchmarkAutoDeposit_bucket_DirectRead measures per-call cost of resolving
// AutoDeposit.bucket(addr) by reading the two storage slots directly (via
// evm.ReadContractStorage), reconstructing the contract's reader logic in
// Go instead of dispatching through the full EVM. This is the candidate
// mitigation path for IIP-59 mainnet activation (see docs/iip-59-perf-report.md):
// AutoDeposit is not upgradeable, so we can't ship a batch view, but the
// contract's storage layout is frozen and cheap to read directly.
//
// Same setup as BenchmarkAutoDeposit_bucket — deploy mainnet runtime bytecode
// via the wrap preamble, seed 30 registered voters, hot-loop rotates through
// them so every read hits the populated branch (registrants=true).
//
// Invocation:
//
//	go test -tags=iip59bench -bench=BenchmarkAutoDeposit_bucket_DirectRead \
//	  -benchmem -count=3 -run=^$ ./action/protocol/execution/
//
// Compare ns/op against BenchmarkAutoDeposit_bucket to estimate the speedup
// available by bypassing SimulateExecution entirely.
func BenchmarkAutoDeposit_bucket_DirectRead(b *testing.B) {
	r := require.New(b)
	ctx := context.Background()

	runtime := loadRuntimeHex(b, "autodeposit_bytecode")
	initBytecode := wrapRuntimeInit(runtime)
	autoDepositABI, err := abi.JSON(strings.NewReader(autoDepositBenchABIJSON))
	r.NoError(err)

	const numVoters = 30

	balances := make([]ExpectedBalance, 0, numVoters+1)
	balances = append(balances, ExpectedBalance{
		Account:    identityset.Address(0).String(),
		RawBalance: "1000000000000000000000000000",
	})
	for i := 1; i <= numVoters; i++ {
		balances = append(balances, ExpectedBalance{
			Account:    identityset.Address(i).String(),
			RawBalance: "1000000000000000000000000000",
		})
	}

	sct := SmartContractTest{
		InitBalances: balances,
		Deployments: []ExecutionConfig{
			{
				ContractIndex: 0,
				RawPrivateKey: identityset.PrivateKey(0).HexString(),
				RawByteCode:   hex.EncodeToString(initBytecode),
				RawAmount:     "0",
				RawGasLimit:   10_000_000,
				RawGasPrice:   "0",
			},
		},
	}

	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg.Genesis.NumSubEpochs = 1
	cfg.Chain.ProducerPrivKey = identityset.PrivateKey(28).HexString()

	bc, sf, dao, ap := sct.prepareBlockchain(ctx, cfg, r)
	defer func() { r.NoError(bc.Stop(ctx)) }()

	contractAddrs := sct.deployContracts(bc, sf, dao, ap, r)
	r.Equal(1, len(contractAddrs))
	contractAddr := contractAddrs[0]
	contractIoAddr, err := address.FromString(contractAddr)
	r.NoError(err)

	regCfgs := make([]*ExecutionConfig, 0, numVoters)
	sameContractAddrs := make([]string, 0, numVoters)
	for i := 1; i <= numVoters; i++ {
		callData, err := autoDepositABI.Pack("register", big.NewInt(int64(i)))
		r.NoError(err)
		regCfgs = append(regCfgs, &ExecutionConfig{
			RawPrivateKey: identityset.PrivateKey(i).HexString(),
			RawByteCode:   hex.EncodeToString(callData),
			RawAmount:     "0",
			RawGasLimit:   500_000,
			RawGasPrice:   "0",
			Comment:       fmt.Sprintf("register voter %d", i),
		})
		sameContractAddrs = append(sameContractAddrs, contractAddr)
	}
	receipts, _, err := sct.runExecutions(bc, sf, dao, ap, regCfgs, sameContractAddrs)
	r.NoError(err)
	r.Equal(numVoters, len(receipts))
	for i, receipt := range receipts {
		r.Equalf(uint64(iotextypes.ReceiptStatus_Success), receipt.Status,
			"register tx %d failed: %s", i, receipt.ExecutionRevertMsg())
	}

	// Pre-compute both storage slot keys per voter so the bench loop only
	// measures the ReadContractStorage path (not keccak/pad overhead —
	// production would amortise these once per voter list too).
	regKeys := make([][]byte, numVoters)
	buckKeys := make([][]byte, numVoters)
	for i := 0; i < numVoters; i++ {
		addr := common.BytesToAddress(identityset.Address(i + 1).Bytes())
		regKeys[i] = mappingSlotKey(addr, autoDepositSlotRegistrants)
		buckKeys[i] = mappingSlotKey(addr, autoDepositSlotBuckets)
	}

	bcCtx, err := bc.Context(context.Background())
	r.NoError(err)
	bcCtx = evm.WithHelperCtx(bcCtx, evm.HelperContext{
		GetBlockHash:   dao.GetBlockHash,
		GetBlockTime:   getBlockTimeForTest,
		DepositGasFunc: rewarding.DepositGas,
	})
	ws, err := sf.WorkingSet(bcCtx)
	r.NoError(err)

	// Sanity check — direct read must return bucket 1 for voter 1,
	// matching what the EVM path returned in the paired bench.
	{
		regRaw, err := evm.ReadContractStorage(bcCtx, ws, contractIoAddr, regKeys[0])
		r.NoError(err)
		r.Equalf(byte(1), regRaw[31], "voter 1 registrants slot = %x, want ...01", regRaw)
		buckRaw, err := evm.ReadContractStorage(bcCtx, ws, contractIoAddr, buckKeys[0])
		r.NoError(err)
		r.Equalf(int64(1), new(big.Int).SetBytes(buckRaw).Int64(),
			"voter 1 buckets slot = %x, want 1", buckRaw)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := i % numVoters
		regRaw, err := evm.ReadContractStorage(bcCtx, ws, contractIoAddr, regKeys[idx])
		if err != nil {
			b.Fatalf("ReadContractStorage registrants: %v", err)
		}
		// Mirror the on-chain gate: only fetch buckets[owner] when
		// registrants[owner]==true. Unregistered voters short-circuit
		// to -1 without touching the second slot.
		if regRaw[31] == 0 {
			continue
		}
		_, err = evm.ReadContractStorage(bcCtx, ws, contractIoAddr, buckKeys[idx])
		if err != nil {
			b.Fatalf("ReadContractStorage buckets: %v", err)
		}
	}
	b.StopTimer()
}

// BenchmarkAutoDeposit_bucket_AdapterReuse measures per-call cost when
// building the StateDBAdapter ONCE per drain and reusing it across every
// voter lookup — the shape a production mitigation would actually take
// (the adapter is a read-only wrapper around the working set, so it's
// safe to hold across voter iterations within a single drain).
//
// This is the tightest lower-bound on "in-process storage lookup" latency
// short of hoisting the state manager's KV interface directly. If this
// number extrapolates within the block budget, mitigation (2) from
// docs/iip-59-perf-report.md is viable without any contract changes.
//
// Invocation:
//
//	go test -tags=iip59bench -bench=BenchmarkAutoDeposit_bucket_AdapterReuse \
//	  -benchmem -count=3 -run=^$ ./action/protocol/execution/
func BenchmarkAutoDeposit_bucket_AdapterReuse(b *testing.B) {
	r := require.New(b)
	ctx := context.Background()

	runtime := loadRuntimeHex(b, "autodeposit_bytecode")
	initBytecode := wrapRuntimeInit(runtime)
	autoDepositABI, err := abi.JSON(strings.NewReader(autoDepositBenchABIJSON))
	r.NoError(err)

	const numVoters = 30
	balances := make([]ExpectedBalance, 0, numVoters+1)
	balances = append(balances, ExpectedBalance{
		Account:    identityset.Address(0).String(),
		RawBalance: "1000000000000000000000000000",
	})
	for i := 1; i <= numVoters; i++ {
		balances = append(balances, ExpectedBalance{
			Account:    identityset.Address(i).String(),
			RawBalance: "1000000000000000000000000000",
		})
	}

	sct := SmartContractTest{
		InitBalances: balances,
		Deployments: []ExecutionConfig{
			{
				ContractIndex: 0,
				RawPrivateKey: identityset.PrivateKey(0).HexString(),
				RawByteCode:   hex.EncodeToString(initBytecode),
				RawAmount:     "0",
				RawGasLimit:   10_000_000,
				RawGasPrice:   "0",
			},
		},
	}

	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg.Genesis.NumSubEpochs = 1
	cfg.Chain.ProducerPrivKey = identityset.PrivateKey(28).HexString()

	bc, sf, dao, ap := sct.prepareBlockchain(ctx, cfg, r)
	defer func() { r.NoError(bc.Stop(ctx)) }()

	contractAddrs := sct.deployContracts(bc, sf, dao, ap, r)
	r.Equal(1, len(contractAddrs))
	contractAddr := contractAddrs[0]

	regCfgs := make([]*ExecutionConfig, 0, numVoters)
	sameContractAddrs := make([]string, 0, numVoters)
	for i := 1; i <= numVoters; i++ {
		callData, err := autoDepositABI.Pack("register", big.NewInt(int64(i)))
		r.NoError(err)
		regCfgs = append(regCfgs, &ExecutionConfig{
			RawPrivateKey: identityset.PrivateKey(i).HexString(),
			RawByteCode:   hex.EncodeToString(callData),
			RawAmount:     "0",
			RawGasLimit:   500_000,
			RawGasPrice:   "0",
			Comment:       fmt.Sprintf("register voter %d", i),
		})
		sameContractAddrs = append(sameContractAddrs, contractAddr)
	}
	receipts, _, err := sct.runExecutions(bc, sf, dao, ap, regCfgs, sameContractAddrs)
	r.NoError(err)
	r.Equal(numVoters, len(receipts))
	for i, receipt := range receipts {
		r.Equalf(uint64(iotextypes.ReceiptStatus_Success), receipt.Status,
			"register tx %d failed: %s", i, receipt.ExecutionRevertMsg())
	}

	// Pre-compute the (regKey, buckKey) pair per voter and convert to
	// go-ethereum common.Hash once, since GetState takes common.Hash.
	regKeys := make([]common.Hash, numVoters)
	buckKeys := make([]common.Hash, numVoters)
	for i := 0; i < numVoters; i++ {
		addr := common.BytesToAddress(identityset.Address(i + 1).Bytes())
		regKeys[i] = common.BytesToHash(mappingSlotKey(addr, autoDepositSlotRegistrants))
		buckKeys[i] = common.BytesToHash(mappingSlotKey(addr, autoDepositSlotBuckets))
	}
	contractEvm := common.BytesToAddress(hashContractAddr(b, contractAddr))

	bcCtx, err := bc.Context(context.Background())
	r.NoError(err)
	bcCtx = evm.WithHelperCtx(bcCtx, evm.HelperContext{
		GetBlockHash:   dao.GetBlockHash,
		GetBlockTime:   getBlockTimeForTest,
		DepositGasFunc: rewarding.DepositGas,
	})
	ws, err := sf.WorkingSet(bcCtx)
	r.NoError(err)

	// Build the adapter ONCE — production would create one per drain
	// and iterate all opted-in voters against it.
	adapter, err := evm.NewStateDBAdapter(ws, 1, hash.ZeroHash256)
	r.NoError(err)

	// Sanity: adapter.GetState must reproduce the same values the
	// EVM path returns. If not, drop the bench: we're not measuring
	// what we think we are.
	{
		reg := adapter.GetState(contractEvm, regKeys[0])
		r.Equalf(byte(1), reg[31], "voter1 registrants via adapter = %x, want ...01", reg)
		buck := adapter.GetState(contractEvm, buckKeys[0])
		r.Equalf(int64(1), new(big.Int).SetBytes(buck[:]).Int64(),
			"voter1 buckets via adapter = %x, want 1", buck)
		buck2 := adapter.GetState(contractEvm, buckKeys[1])
		r.Equalf(int64(2), new(big.Int).SetBytes(buck2[:]).Int64(),
			"voter2 buckets via adapter = %x, want 2", buck2)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := i % numVoters
		reg := adapter.GetState(contractEvm, regKeys[idx])
		if reg[31] == 0 {
			continue
		}
		_ = adapter.GetState(contractEvm, buckKeys[idx])
	}
	b.StopTimer()
}

// hashContractAddr converts an iotex bech32 contract address string into
// the raw 20-byte form that go-ethereum's common.Address expects. The
// StateDBAdapter internally hashes the common.Address the same way its
// StateDB writers did, so this must match how the EVM saw the contract
// during deploy.
func hashContractAddr(tb testing.TB, ioAddr string) []byte {
	tb.Helper()
	a, err := address.FromString(ioAddr)
	if err != nil {
		tb.Fatalf("parse contract addr: %v", err)
	}
	return a.Bytes()
}

// BenchmarkAutoDeposit_bucket_WrapperContract measures per-voter cost when
// resolving AutoDeposit.bucket(addr) via a locally-deployed wrapper contract
// (AutoDepositBatch, source at e2etest/autodeposit_batch.sol). The wrapper
// exposes `buckets(address[]) → int256[]` which for-loops STATICCALL to the
// real AutoDeposit for each address; a single SimulateExecution against the
// wrapper amortises the EVM-setup cost that dominates the per-call baseline
// (~65μs).
//
// This is mitigation option (1a) from docs/iip-59-perf-report.md — the
// wrapper-contract fallback if the direct-storage-read mitigation isn't
// viable. Report ns/op is the total call cost for a numVoters-sized batch;
// divide by numVoters (30) for per-voter cost.
//
// Invocation:
//
//	go test -tags=iip59bench -bench=BenchmarkAutoDeposit_bucket_WrapperContract \
//	  -benchmem -count=3 -run=^$ ./action/protocol/execution/
func BenchmarkAutoDeposit_bucket_WrapperContract(b *testing.B) {
	r := require.New(b)
	ctx := context.Background()

	autoDepositRuntime := loadRuntimeHex(b, "autodeposit_bytecode")
	autoDepositInit := wrapRuntimeInit(autoDepositRuntime)
	autoDepositABI, err := abi.JSON(strings.NewReader(autoDepositBenchABIJSON))
	r.NoError(err)
	batchABI, err := abi.JSON(strings.NewReader(autoDepositBatchABIJSON))
	r.NoError(err)

	// Wrapper contract init bytecode was compiled locally from
	// e2etest/autodeposit_batch.sol with solc 0.8.33. Unlike the
	// mainnet AutoDeposit fixture (runtime bytecode only), this is
	// full init bytecode — the constructor sets the immutable
	// `target` from the ABI-encoded address we append below.
	batchInitHex := strings.TrimSpace(string(loadFile(b, "autodeposit_batch_init_bytecode")))
	batchInit, err := hex.DecodeString(batchInitHex)
	r.NoError(err)

	const numVoters = 30

	balances := make([]ExpectedBalance, 0, numVoters+1)
	balances = append(balances, ExpectedBalance{
		Account:    identityset.Address(0).String(),
		RawBalance: "1000000000000000000000000000",
	})
	for i := 1; i <= numVoters; i++ {
		balances = append(balances, ExpectedBalance{
			Account:    identityset.Address(i).String(),
			RawBalance: "1000000000000000000000000000",
		})
	}

	// First deploy: AutoDeposit (wrapped runtime → CREATE).
	sct := SmartContractTest{
		InitBalances: balances,
		Deployments: []ExecutionConfig{
			{
				ContractIndex: 0,
				RawPrivateKey: identityset.PrivateKey(0).HexString(),
				RawByteCode:   hex.EncodeToString(autoDepositInit),
				RawAmount:     "0",
				RawGasLimit:   10_000_000,
				RawGasPrice:   "0",
			},
		},
	}

	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg.Genesis.NumSubEpochs = 1
	cfg.Chain.ProducerPrivKey = identityset.PrivateKey(28).HexString()

	bc, sf, dao, ap := sct.prepareBlockchain(ctx, cfg, r)
	defer func() { r.NoError(bc.Stop(ctx)) }()

	contractAddrs := sct.deployContracts(bc, sf, dao, ap, r)
	r.Equal(1, len(contractAddrs))
	autoDepositAddr := contractAddrs[0]
	autoDepositIo, err := address.FromString(autoDepositAddr)
	r.NoError(err)

	// Seed AutoDeposit with 30 registered voters.
	regCfgs := make([]*ExecutionConfig, 0, numVoters)
	sameAddrs := make([]string, 0, numVoters)
	for i := 1; i <= numVoters; i++ {
		callData, err := autoDepositABI.Pack("register", big.NewInt(int64(i)))
		r.NoError(err)
		regCfgs = append(regCfgs, &ExecutionConfig{
			RawPrivateKey: identityset.PrivateKey(i).HexString(),
			RawByteCode:   hex.EncodeToString(callData),
			RawAmount:     "0",
			RawGasLimit:   500_000,
			RawGasPrice:   "0",
			Comment:       fmt.Sprintf("register voter %d", i),
		})
		sameAddrs = append(sameAddrs, autoDepositAddr)
	}
	receipts, _, err := sct.runExecutions(bc, sf, dao, ap, regCfgs, sameAddrs)
	r.NoError(err)
	r.Equal(numVoters, len(receipts))
	for i, receipt := range receipts {
		r.Equalf(uint64(iotextypes.ReceiptStatus_Success), receipt.Status,
			"register tx %d failed: %s", i, receipt.ExecutionRevertMsg())
	}

	// Second deploy: AutoDepositBatch with AutoDeposit as constructor arg.
	// SimulateExecution can't deploy contracts; use runExecutions with an
	// empty target (contract-creation tx).
	ctorArg := make([]byte, 32)
	copy(ctorArg[12:32], autoDepositIo.Bytes())
	batchDeployCode := append([]byte{}, batchInit...)
	batchDeployCode = append(batchDeployCode, ctorArg...)
	deployCfg := &ExecutionConfig{
		RawPrivateKey: identityset.PrivateKey(0).HexString(),
		RawByteCode:   hex.EncodeToString(batchDeployCode),
		RawAmount:     "0",
		RawGasLimit:   10_000_000,
		RawGasPrice:   "0",
		Comment:       "deploy AutoDepositBatch",
	}
	deployReceipts, _, err := sct.runExecutions(bc, sf, dao, ap,
		[]*ExecutionConfig{deployCfg}, []string{""})
	r.NoError(err)
	r.Equal(1, len(deployReceipts))
	r.Equalf(uint64(iotextypes.ReceiptStatus_Success), deployReceipts[0].Status,
		"batch deploy failed: %s", deployReceipts[0].ExecutionRevertMsg())
	batchAddr := deployReceipts[0].ContractAddress
	r.NotEmpty(batchAddr, "batch contract address must be populated")

	// Pack one batch call covering all numVoters voters.
	voterEvmAddrs := make([]common.Address, numVoters)
	for i := 0; i < numVoters; i++ {
		voterEvmAddrs[i] = common.BytesToAddress(identityset.Address(i + 1).Bytes())
	}
	batchCallData, err := batchABI.Pack("buckets", voterEvmAddrs)
	r.NoError(err)

	bcCtx, err := bc.Context(context.Background())
	r.NoError(err)
	bcCtx = evm.WithHelperCtx(bcCtx, evm.HelperContext{
		GetBlockHash:   dao.GetBlockHash,
		GetBlockTime:   getBlockTimeForTest,
		DepositGasFunc: rewarding.DepositGas,
	})
	ws, err := sf.WorkingSet(bcCtx)
	r.NoError(err)
	zeroAddr, err := address.FromString(address.ZeroAddress)
	r.NoError(err)

	// Sanity check — batch call must return [1, 2, ..., 30] matching the
	// register(i) seed. If not, ABI encoding or immutable-target patching
	// went wrong and we'd measure the wrong path.
	{
		ex := action.NewExecution(batchAddr, big.NewInt(0), batchCallData)
		elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(50_000_000).SetAction(ex).Build()
		ret, _, err := evm.SimulateExecution(bcCtx, ws, zeroAddr, elp)
		r.NoError(err)
		unpacked, err := batchABI.Unpack("buckets", ret)
		r.NoError(err)
		r.Equal(1, len(unpacked))
		vals := unpacked[0].([]*big.Int)
		r.Equalf(numVoters, len(vals), "expected %d results, got %d", numVoters, len(vals))
		for i, v := range vals {
			r.Equalf(int64(i+1), v.Int64(),
				"voter %d bucket = %s, want %d (batch call wrong path?)", i+1, v.String(), i+1)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ex := action.NewExecution(batchAddr, big.NewInt(0), batchCallData)
		elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(50_000_000).SetAction(ex).Build()
		_, _, err := evm.SimulateExecution(bcCtx, ws, zeroAddr, elp)
		if err != nil {
			b.Fatalf("SimulateExecution: %v", err)
		}
	}
	b.StopTimer()

	// Report per-voter cost as an extra metric so the raw ns/op (which
	// is total-batch cost) is easy to compare against the per-call
	// baselines from the other benches.
	if b.N > 0 {
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N)/float64(numVoters), "ns/voter")
	}
}

// loadFile reads an e2etest fixture verbatim (no hex decode). Used for
// bytecode fixtures that this bench appends constructor args to.
func loadFile(tb testing.TB, name string) []byte {
	tb.Helper()
	path := "../../../e2etest/" + name
	raw, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("read fixture %s: %v", path, err)
	}
	return raw
}
