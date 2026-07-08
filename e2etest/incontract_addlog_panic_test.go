package e2etest

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// panicInitCode returns constructor bytecode that emits LOG1 with topic0 == 0 (the
// reserved in-contract-transfer marker) and only ONE topic, then returns empty
// runtime code. Because the topic count is not exactly 3, StateDBAdapter.AddLog hits
// the reserved-topic branch with a malformed topic count.
//
//	PUSH32 <100 IOTX> PUSH1 0 MSTORE   ; mem[0:32] = amount (log data)
//	PUSH1 0 PUSH1 32 PUSH1 0 LOG1       ; LOG1 topic0=0, len=32, offset=0
//	PUSH1 0 PUSH1 0 RETURN              ; return empty runtime code
func panicInitCode(t *testing.T) []byte {
	code, err := hex.DecodeString(
		"7f0000000000000000000000000000000000000000000000056bc75e2d63100000" +
			"600052600060206000a160006000f3")
	require.NoError(t, err)
	return code
}

func signPanicDeploy(t *testing.T) *action.SealedEnvelope {
	exec := action.NewExecution(action.EmptyAddress, big.NewInt(0), panicInitCode(t))
	elp := (&action.EnvelopeBuilder{}).SetAction(exec).SetNonce(1).SetGasLimit(10000000).
		SetGasPrice(big.NewInt(9000000000000)).Build()
	priKey, err := crypto.HexStringToPrivateKey(_executorPriKey)
	require.NoError(t, err)
	selp, err := action.Sign(elp, priKey)
	require.NoError(t, err)
	return selp
}

// TestInContractTransferAddLogPanicPreZanzibar drives a contract whose init code
// emits a malformed reserved-topic LOG1 (topic0 == 0, one topic) through the full
// block pipeline on a chain where the Zanzibar fork is NOT yet active. This is the
// preserved historical behavior: StateDBAdapter.AddLog panics ("Invalid in contract
// transfer topics") on a reserved-topic log whose topic count != 3. The mint draft
// recover turns that panic into a mint error, so the doomed block is never produced
// (a contract cannot mint the tx into a canonical block pre-fork). This determinism
// is required to replay historical blocks byte-for-byte.
func TestInContractTransferAddLogPanicPreZanzibar(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	// Default genesis keeps ZanzibarBlockHeight at its MaxUint64 placeholder, so the
	// fork is inactive and the historical panic path is exercised.
	bc, _, ap := prepareBlockchain(ctx, _executor, r)
	defer r.NoError(bc.Stop(ctx))
	ctx = genesis.WithGenesisContext(ctx, bc.Genesis())

	_, receipt, blk, err := addOneTx(ctx, ap, bc, &actionWithTime{signPanicDeploy(t), testutil.TimestampNow()})
	// Pre-fork: the malformed reserved-topic log makes mint panic; the mint recover
	// converts it into an error and no block/receipt is produced.
	r.Error(err)
	r.Nil(receipt)
	r.Nil(blk)
}

// TestInContractTransferAddLogPanicPostZanzibar drives the same malformed
// reserved-topic LOG1 through the full block pipeline on a chain where the Zanzibar
// fork IS active. Post-fork, AddLog drops the log for any topic count and never
// panics, so the block is minted, applied and committed normally, the receipt is
// Success, no forged IN_CONTRACT_TRANSFER transaction log is recorded, and
// receipt.Logs is unaffected (the reserved-topic log never enters it).
func TestInContractTransferAddLogPanicPostZanzibar(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	// Activate Zanzibar from height 1 so the very first block is post-fork.
	bc, _, ap := prepareBlockchainWithGenesis(ctx, _executor, r, func(g *genesis.Genesis) {
		g.ZanzibarBlockHeight = 1
	})
	defer r.NoError(bc.Stop(ctx))
	ctx = genesis.WithGenesisContext(ctx, bc.Genesis())

	// Must NOT panic: the tx flows through block draft AND block apply/validation.
	_, receipt, blk, err := addOneTx(ctx, ap, bc, &actionWithTime{signPanicDeploy(t), testutil.TimestampNow()})
	r.NoError(err)
	r.NotNil(receipt)
	r.NotNil(blk)
	r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)

	// The malformed reserved-topic log is dropped, not turned into a tx log, and does
	// not enter receipt.Logs.
	forged := 0
	for _, tl := range receipt.TransactionLogs() {
		if tl.Type == iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER {
			forged++
		}
	}
	r.Zerof(forged, "node must not record a forged IN_CONTRACT_TRANSFER (found %d)", forged)
	r.Empty(receipt.Logs(), "malformed reserved-topic log must not enter receipt.Logs")
}
