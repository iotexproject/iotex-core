package e2etest

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// Deploys the EXACT constructor bytecode from mainnet tx
// 0xadd4b0f5930e7acc56a06ae585a91ef60da997d7c3f8075a1be7a3f0fe4eec06 through the
// full block pipeline, and asserts the node does not record a forged
// IN_CONTRACT_TRANSFER transaction log, and the "recipient" balance is unchanged.
//
// The init code emits LOG3 with topic0 = 0 (the reserved in-contract-transfer
// marker), topic1 = 0xce17..520e2 (forged sender), topic2 = 0x986895eb..209c
// (forged recipient), data = 100 IOTX, then returns empty runtime code.
// On an unpatched node this produces an IN_CONTRACT_TRANSFER of 100 IOTX.
func TestInContractTransferForgeryDeploy(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	bc, sf, ap := prepareBlockchain(ctx, _executor, r)
	defer r.NoError(bc.Stop(ctx))
	ctx = genesis.WithGenesisContext(ctx, bc.Genesis())

	forgeryInitCode, err := hex.DecodeString(
		"7f0000000000000000000000000000000000000000000000056bc75e2d63100000" +
			"6000527f000000000000000000000000986895eb8a117af83258e28df92d8fcb5acb209c" +
			"7f000000000000000000000000ce17cfc932c978a374a9373d89edd18ce9b520e2" +
			"600060206000a360006000f3")
	r.NoError(err)

	victim, err := address.FromHex("986895eb8a117af83258e28df92d8fcb5acb209c")
	r.NoError(err)
	balBefore, err := accountutil.AccountState(ctx, sf, victim)
	r.NoError(err)

	exec := action.NewExecution(action.EmptyAddress, big.NewInt(0), forgeryInitCode)
	elp := (&action.EnvelopeBuilder{}).SetAction(exec).SetNonce(1).SetGasLimit(10000000).
		SetGasPrice(big.NewInt(9000000000000)).Build()
	priKey, err := crypto.HexStringToPrivateKey(_executorPriKey)
	r.NoError(err)
	selp, err := action.Sign(elp, priKey)
	r.NoError(err)

	_, receipt, _, err := addOneTx(ctx, ap, bc, &actionWithTime{selp, testutil.TimestampNow()})
	r.NoError(err)
	r.NotNil(receipt)
	r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)

	forged := 0
	for _, tl := range receipt.TransactionLogs() {
		t.Logf("txlog type=%v sender=%s recipient=%s amount=%s", tl.Type, tl.Sender, tl.Recipient, tl.Amount)
		if tl.Type == iotextypes.TransactionLogType_IN_CONTRACT_TRANSFER {
			forged++
		}
	}
	r.Zerof(forged, "node must not record a forged IN_CONTRACT_TRANSFER (found %d)", forged)

	balAfter, err := accountutil.AccountState(ctx, sf, victim)
	r.NoError(err)
	r.Zero(balAfter.Balance.Cmp(balBefore.Balance), "victim balance must be unchanged")
	t.Logf("victim balance before=%s after=%s", balBefore.Balance, balAfter.Balance)
}
