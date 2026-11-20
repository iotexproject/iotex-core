package staking

// Tests for CandidateEndorsement actions whose op is outside the defined range
// (4..255 after the uint8 conversion).
//
// Such an action is rejected at admission (Protocol.Validate, the
// ActionValidator the pool and the mint loop run before execution). If one
// still reaches execution, the handler settles a failure receipt that charges
// gas and spends the nonce, instead of returning an error that discards the
// draft block.

import (
	"context"
	"math/big"
	"testing"

	"github.com/mohae/deepcopy"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// endorsementOpCtx builds a post-Upernavik feature context (current mainnet rules:
// EnforceLegacyEndorsement=false, DisableDelegateEndorsement=false).
func endorsementOpCtx(t *testing.T, nonce uint64) context.Context {
	t.Helper()
	ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
		Caller:       identityset.Address(1),
		GasPrice:     big.NewInt(1000),
		IntrinsicGas: action.CandidateEndorsementBaseIntrinsicGas,
		Nonce:        nonce,
	})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight:    1,
		BlockTimeStamp: timeBlock,
		GasLimit:       1000000,
	})
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{}})
	cfg := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
	cfg.GreenlandBlockHeight = 1
	cfg.TsunamiBlockHeight = 1
	cfg.UpernavikBlockHeight = 1
	ctx = genesis.WithGenesisContext(ctx, cfg)
	return protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
}

// endorsementWithOp builds a signed CandidateEndorsement with the given wire op,
// decoded from the protobuf so that ops outside the defined range can be set.
func endorsementWithOp(t *testing.T, bucketIndex uint64, op uint32) (*action.CandidateEndorsement, action.Envelope) {
	t.Helper()
	var act action.CandidateEndorsement
	require.NoError(t, act.LoadProto(&iotextypes.CandidateEndorsement{
		BucketIndex: bucketIndex,
		Endorse:     true,
		Op:          op,
	}))
	elp := builder.SetNonce(1).SetGasLimit(1000000).
		SetGasPrice(big.NewInt(1000)).SetAction(&act).Build()
	return &act, elp
}

func endorsementOpSetup(t *testing.T, ctrl *gomock.Controller) (protocol.StateManager, *Protocol, uint64) {
	t.Helper()
	initBucketCfgs := []*bucketConfig{
		{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 30, true, false, nil, 0},
	}
	initCandidateCfgs := []*candidateConfig{
		{identityset.Address(1), identityset.Address(7), identityset.Address(1), "test1"},
	}
	sm, p, buckets, _ := initTestState(t, ctrl, initBucketCfgs, initCandidateCfgs)
	require.NoError(t, setupAccount(sm, identityset.Address(1), 1300000))
	return sm, p, buckets[0].Index
}

// Admission rejects an out-of-range op, so the action never enters a draft.
func TestCandidateEndorsementOutOfRangeOpRejectedAtAdmission(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm, p, bucketIdx := endorsementOpSetup(t, ctrl)
	ctx := endorsementOpCtx(t, 1)

	for _, op := range []uint32{4, 42, 128, 254, 255} {
		_, elp := endorsementWithOp(t, bucketIdx, op)
		r.Error(p.Validate(ctx, elp, sm), "admission must reject op=%d", op)
	}
	// Sanity: the defined ops are still admitted.
	for _, op := range []uint32{1, 2, 3} {
		_, elp := endorsementWithOp(t, bucketIdx, op)
		r.NoError(p.Validate(ctx, elp, sm), "admission must keep accepting op=%d", op)
	}
}

// Exhaustive: every op in [4, 255] is rejected at admission.
func TestCandidateEndorsementOutOfRangeOpRejectedAtAdmissionAllOps(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm, p, bucketIdx := endorsementOpSetup(t, ctrl)
	ctx := endorsementOpCtx(t, 1)

	for op := 4; op <= 255; op++ {
		_, elp := endorsementWithOp(t, bucketIdx, uint32(op))
		r.Error(p.Validate(ctx, elp, sm), "admission must reject op=%d", op)
	}
}

// If an out-of-range op reaches execution anyway, for example in a block from
// another proposer or a pool entry persisted before an upgrade, the handler
// settles a failure receipt that charges gas and spends the nonce, and the
// draft is kept.
func TestCandidateEndorsementOutOfRangeOpSettlesFailureReceipt(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm, p, bucketIdx := endorsementOpSetup(t, ctrl)
	ctx := endorsementOpCtx(t, 1)

	act, elp := endorsementWithOp(t, bucketIdx, 4)
	receipt, err := p.Handle(ctx, elp, sm)
	r.NoError(err, "handler must settle a receipt, not return an error")
	r.NotNil(receipt, "an out-of-range op must still produce a receipt")
	r.Equal(uint64(iotextypes.ReceiptStatus_Failure), receipt.Status)
	r.Greater(receipt.GasConsumed, uint64(0), "the failed action must still charge gas")
	r.NoError(act.SanityCheck())
}

// CandidateDeactivate carries an op field too, and its handler already settles
// a failure receipt for an out-of-range op; pin that behavior.
func TestCandidateDeactivateOutOfRangeOpSettlesFailureReceipt(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// The deactivate handler requires the caller to own a candidate with a
	// self-stake bucket before it reaches the op switch.
	initBucketCfgs := []*bucketConfig{
		{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 30, true, true, nil, 0},
	}
	initCandidateCfgs := []*candidateConfig{
		{identityset.Address(1), identityset.Address(7), identityset.Address(1), "test1"},
	}
	sm, p, _, _ := initTestState(t, ctrl, initBucketCfgs, initCandidateCfgs)
	require.NoError(t, setupAccount(sm, identityset.Address(1), 1300000))

	deact := action.NewCandidateDeactivate(action.CandidateDeactivateOp(7))
	elp := builder.SetNonce(1).SetGasLimit(1000000).
		SetGasPrice(big.NewInt(1000)).SetAction(deact).Build()

	receipt, err := p.Handle(endorsementOpCtx(t, 1), elp, sm)
	r.NoError(err, "deactivate handler must fail soft for an out-of-range op")
	r.NotNil(receipt)
	r.Equal(uint64(iotextypes.ReceiptStatus_Failure), receipt.Status)
}

// The wire op field is uint32 but the Go type is uint8, so ops above 255
// truncate: values truncating to 0..3 are well-defined actions, and values
// truncating to 4..255 are rejected at admission.
func TestCandidateEndorsementOpAbove255Truncation(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm, p, bucketIdx := endorsementOpSetup(t, ctrl)
	ctx := endorsementOpCtx(t, 1)

	// 257 truncates to 1 (Endorse): a well-defined action, admitted.
	_, elp := endorsementWithOp(t, bucketIdx, 257)
	r.NoError(p.Validate(ctx, elp, sm))

	// 300 truncates to 44: rejected at admission.
	_, elp = endorsementWithOp(t, bucketIdx, 300)
	r.Error(p.Validate(ctx, elp, sm))
}
