// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

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
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/autodeposit"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestSlotBucketReader_MatchesContract is the sanity guard for the
// direct-slot AutoDeposit path. It deploys mainnet AutoDeposit runtime
// bytecode, registers two voters via real register() txs, and asserts that
// autodeposit.SlotBucketReader.LookupBucket returns the exact bucket ids
// the contract itself wrote. A failure means either the slot constants in
// action/protocol/rewarding/autodeposit/slot_reader.go have drifted, or
// evm.NewSlotReader's setup no longer matches the EVM's working set — in
// either case the production drain would return wrong bucket ids without
// this test. See docs/iip-59-perf-report.md for the mitigation this guards.
func TestSlotBucketReader_MatchesContract(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()

	runtime := loadAutoDepositRuntime(t, "autodeposit_bytecode")
	initBytecode := wrapAutoDepositInit(runtime)
	autoDepositABI, err := abi.JSON(strings.NewReader(autoDepositSanityABIJSON))
	r.NoError(err)

	const numVoters = 2
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

	// Register voter i with bucketId=i so we can disambiguate a
	// mis-slotted lookup that returns the wrong voter's id.
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

	bcCtx, err := bc.Context(context.Background())
	r.NoError(err)
	bcCtx = evm.WithHelperCtx(bcCtx, evm.HelperContext{
		GetBlockHash:   dao.GetBlockHash,
		GetBlockTime:   getBlockTimeForTest,
		DepositGasFunc: rewarding.DepositGas,
	})
	ws, err := sf.WorkingSet(bcCtx)
	r.NoError(err)

	slotReader, err := evm.NewSlotReader(bcCtx, ws)
	r.NoError(err)

	bridge, err := autodeposit.New(contractAddr)
	r.NoError(err)
	bucketReader, err := bridge.NewSlotBucketReader(slotReader)
	r.NoError(err)

	// Registered voters must round-trip through the direct-slot path.
	for i := 1; i <= numVoters; i++ {
		voter := identityset.Address(i)
		gotID, present, err := bucketReader.LookupBucket(voter)
		r.NoError(err, "voter %d", i)
		r.Truef(present, "voter %d: expected registered", i)
		r.Equalf(uint64(i), gotID, "voter %d: bucket id mismatch", i)
	}

	// A never-registered voter must fall through to (0, false, nil) so the
	// caller routes the payout to credit instead of the compound path.
	unreg := identityset.Address(numVoters + 1)
	gotID, present, err := bucketReader.LookupBucket(unreg)
	r.NoError(err)
	r.False(present, "unregistered voter must not be marked present")
	r.Zero(gotID)
}

// autoDepositSanityABIJSON exposes only register(int256) — the minimum
// surface to seed on-chain state for this test. Kept private and distinct
// from the bench file's ABI JSON so the two files can coexist under the
// iip59bench build tag without symbol collisions.
const autoDepositSanityABIJSON = `[
    {"inputs":[{"internalType":"int256","name":"bucketId","type":"int256"}],"name":"register","outputs":[],"stateMutability":"nonpayable","type":"function"}
]`

// loadAutoDepositRuntime mirrors the bench file's loadRuntimeHex helper.
// Duplicated (not shared) so this untagged test doesn't depend on symbols
// that only exist under the iip59bench build tag, and so the two files can
// coexist when both tags are enabled.
func loadAutoDepositRuntime(tb testing.TB, name string) []byte {
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

// wrapAutoDepositInit prepends the 12-byte CODECOPY+RETURN preamble so
// runtime bytecode (which lacks a constructor prefix) can be installed by
// a normal contract-creation tx. Byte-identical to wrapRuntimeInit in
// protocol_iip59_bench_test.go — the layout is fixed by the EVM.
func wrapAutoDepositInit(runtime []byte) []byte {
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
