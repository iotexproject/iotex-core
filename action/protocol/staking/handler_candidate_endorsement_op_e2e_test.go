package staking_test

// End-to-end tests for a CandidateEndorsement with an out-of-range op, run over
// a real state factory on both sides of the RevertStakingStateOnFailedReceipt
// gate (Zanzibar Gamma). The action settles a failure receipt that charges gas
// and spends the nonce, so resubmitting the same bytes fails admission with
// ErrNonceTooLow.

import (
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

func TestCandidateEndorsementOutOfRangeOpEndToEnd(t *testing.T) {
	for _, gate := range []struct {
		name   string
		height uint64
	}{
		{"pre-gate", math.MaxUint64},
		{"post-gate", 1},
	} {
		t.Run(gate.name, func(t *testing.T) {
			r := require.New(t)
			e := newStakingEnv(t, gate.height)
			e.setupBucket(t) // registers cand1; bucket 1 belongs to _staker

			var act action.CandidateEndorsement
			r.NoError(act.LoadProto(&iotextypes.CandidateEndorsement{
				BucketIndex: 1,
				Endorse:     true,
				Op:          4,
			}))

			balanceBefore := new(big.Int).Set(e.account(t, _staker).Balance)
			nonceBefore := e.account(t, _staker).PendingNonce()

			// e.run asserts NoError and a non-nil receipt.
			receipt := e.run(t, _staker, 3, envelope(3).SetAction(&act).Build(), time.Unix(e.g.Timestamp, 0).Add(time.Hour))
			r.EqualValues(iotextypes.ReceiptStatus_Failure, receipt.Status)

			// The sender pays gas and the nonce is spent, so resubmitting the
			// same bytes fails admission with ErrNonceTooLow.
			acc := e.account(t, _staker)
			gasFee := new(big.Int).Mul(_testGasPrice, new(big.Int).SetUint64(receipt.GasConsumed))
			r.Equal(new(big.Int).Sub(balanceBefore, gasFee), acc.Balance)
			r.Equal(nonceBefore+1, acc.PendingNonce())
		})
	}
}

// The admission layer must reject an out-of-range op before the action can
// enter a pool or a draft: with the wire-level op at 300, Protocol.Validate
// fails while a well-defined op (1..3) still passes. Raw op=300 truncates to
// 44 in the uint8 field, which the range check catches.
func TestCandidateEndorsementOutOfRangeOpEndToEndAdmission(t *testing.T) {
	r := require.New(t)
	e := newStakingEnv(t, math.MaxUint64)
	e.setupBucket(t)

	var act action.CandidateEndorsement
	r.NoError(act.LoadProto(&iotextypes.CandidateEndorsement{
		BucketIndex: 1,
		Endorse:     true,
		Op:          300,
	}))
	elp := envelope(3).SetAction(&act).Build()

	ctx := e.actionCtx(_staker, 3, action.CandidateEndorsementBaseIntrinsicGas, time.Unix(e.g.Timestamp, 0).Add(time.Hour))
	r.ErrorIs(e.p.Validate(ctx, elp, e.ws), action.ErrInvalidAct)
}
