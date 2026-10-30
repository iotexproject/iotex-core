package e2etest

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// TestSelfDestructTransactionLogAmount is a regression guard for a bug where the
// IN_CONTRACT_TRANSFER transaction log recorded for a SELFDESTRUCT carried the wrong
// amount. generateSelfDestructTransferLog stored stateDB.lastAddBalanceAmount (a
// *big.Int) directly into the log. That big.Int is mutated in place by every later
// AddBalance — in particular the gas-deposit refund that ExecuteContract performs
// immediately after the EVM returns (stateDB.AddBalance(origin, depositGas*gasPrice)).
// So the already-recorded self-destruct log was retroactively overwritten with the
// gas-refund value instead of the amount actually transferred to the beneficiary.
//
// The contract below mirrors the on-chain MEV pattern that triggered this on mainnet:
// it makes a value-carrying sub-call that reverts (an arbitrage probe) and then
// self-destructs its own balance. The self-destruct moves V to the beneficiary, so the
// transaction log must report V — not the (much larger) gas-deposit refund.
func TestSelfDestructTransactionLogAmount(t *testing.T) {
	r := require.New(t)
	deployer := identityset.Address(10).String()
	deployerSK := identityset.PrivateKey(10)

	cfg := initCfg(r)
	// initCfg uses config.Default's fixed API ports; randomize so this test does
	// not collide with the fixed :14014 gRPC port of a sibling e2e test.
	cfg.API.GRPCPort = testutil.RandomPort()
	cfg.API.HTTPPort = testutil.RandomPort()
	cfg.API.WebSocketPort = 0
	cfg.Genesis.SumatraBlockHeight = 1
	cfg.Genesis.UpernavikBlockHeight = 1
	cfg.Genesis.VanuatuBlockHeight = 1 // activates Cancun / EIP-6780
	cfg.Genesis.InitBalanceMap[deployer] = unit.ConvertIotxToRau(2000000).String()
	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	test := newE2ETest(t, cfg)
	defer test.teardown()

	bc := test.cs.Blockchain()
	ap := test.cs.ActionPool()
	chainID := cfg.Chain.ID
	gasPrice := big.NewInt(unit.Qev)
	nonce := uint64(0) // Sumatra active -> fresh account uses zero-nonce
	hx := func(s string) []byte { b, err := hex.DecodeString(s); r.NoError(err); return b }
	sign := func(exec *action.Execution) *actionWithTime {
		tx := action.NewLegacyTx(chainID, nonce, gasLimit, gasPrice)
		nonce++
		return &actionWithTime{mustNoErr(action.Sign(action.NewEnvelope(tx, exec), deployerSK)), testutil.TimestampNow()}
	}
	ctx := context.Background()

	// R: always reverts on call (runtime: PUSH1 0 PUSH1 0 REVERT)
	_, rc, _, err := addOneTx(ctx, ap, bc, sign(action.NewExecution("", big.NewInt(0), hx("600580600b6000396000f360006000fd"))))
	r.NoError(err)
	r.EqualValues(iotextypes.ReceiptStatus_Success, rc.Status)
	rAddr := mustNoErr(address.FromString(rc.ContractAddress)).Bytes()

	// P: CALL(R, value=7) which reverts, then SELFDESTRUCT to CALLER.
	//   PUSH1 0 x4 (ret/args off+size), PUSH1 7 (value), PUSH20 R, GAS, CALL, POP, CALLER, SELFDESTRUCT
	pRuntime := append(append(hx("6000600060006000"+"6007"+"73"), rAddr...), hx("5af15033ff")...)
	pInit := append([]byte{0x60, byte(len(pRuntime)), 0x80, 0x60, 0x0b, 0x60, 0x00, 0x39, 0x60, 0x00, 0xf3}, pRuntime...)
	_, pc, _, err := addOneTx(ctx, ap, bc, sign(action.NewExecution("", big.NewInt(0), pInit)))
	r.NoError(err)
	r.EqualValues(iotextypes.ReceiptStatus_Success, pc.Status)
	pAddr := pc.ContractAddress

	// Call P with value V in a batch of 3 within a single minted block (the on-chain
	// pattern batches many such calls; they must not share/leak a corrupted amount).
	V := big.NewInt(100)
	txs := []*actionWithTime{sign(action.NewExecution(pAddr, V, nil)), sign(action.NewExecution(pAddr, V, nil)), sign(action.NewExecution(pAddr, V, nil))}
	_, receipts, _, err := runTxs(ctx, ap, bc, txs)
	r.NoError(err)

	for i, rcpt := range receipts {
		r.NotNilf(rcpt, "receipt %d", i)
		r.EqualValuesf(iotextypes.ReceiptStatus_Success, rcpt.Status, "call %d", i)
		var found bool
		for _, l := range rcpt.TransactionLogs() {
			if l.Type == iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER && l.Sender == pAddr {
				found = true
				// the self-destruct moved exactly V to the beneficiary
				r.Equalf(0, V.Cmp(l.Amount), "call %d: self-destruct log amount = %s, want %s", i, l.Amount, V)
			}
		}
		r.Truef(found, "call %d: missing self-destruct transaction log", i)
	}
}
