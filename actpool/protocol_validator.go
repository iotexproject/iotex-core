// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

// protocolValidator runs the registered protocols' own validation rules at
// admission, so an action the protocol will certainly reject is refused by
// eth_sendRawTransaction instead of being accepted into the pool.
//
// Without it the pool applies only GenericValidator -- nonce, balance,
// signature -- and a feature-gated action that no proposer can ever include
// still takes a nonce and holds it until ActionExpiry. Because nonces must be
// consecutive, that one action stalls every later action from the same account,
// with no receipt and no log to say why. The official CLI reaches this: before
// Zanzibar, `ioctl stake2 update` without --bls-* flags builds exactly such an
// action.
//
// This validator is deliberately admission-only. It must be registered as a
// private validator, never via AddActionEnvelopeValidators: those also run in
// actPool.Validate, which block validation reaches through the bundle pool.
// Rejecting there would turn "included in a block with a failure receipt" into
// "block is invalid", which is a consensus change -- see the comment above
// validateBLSPairing in action/protocol/staking/validations.go.
type protocolValidator struct {
	// reg is held as a pointer and read at Validate time, not at construction.
	// The action pool is built before the protocols are registered
	// (chainservice/builder.go builds the pool, then registers), so resolving
	// eagerly would capture an empty registry.
	reg          *protocol.Registry
	sr           protocol.StateReader
	chainContext func(context.Context) (context.Context, error)
}

// NewProtocolValidator returns a validator that applies every registered
// protocol's ActionValidator rules to an incoming action.
func NewProtocolValidator(reg *protocol.Registry, sr protocol.StateReader, chainContext ...func(context.Context) (context.Context, error)) action.SealedEnvelopeValidator {
	v := &protocolValidator{reg: reg, sr: sr}
	if len(chainContext) > 0 {
		v.chainContext = chainContext[0]
	}
	return v
}

func (v *protocolValidator) Validate(ctx context.Context, selp *action.SealedEnvelope) error {
	// System actions are produced by the proposer, never submitted, and several
	// of them do not carry a sender the ActionCtx below could be built from.
	if action.IsSystemAction(selp) {
		return nil
	}
	ctx, err := withActionCtx(ctx, selp)
	if err != nil {
		return err
	}
	for _, p := range v.reg.All() {
		validator, ok := p.(protocol.ActionValidator)
		if !ok {
			continue
		}
		if err := validator.Validate(ctx, selp.Envelope, v.sr); err != nil {
			return err
		}
	}
	// Inspect a copy: unfolding must not change the admitted transaction's hash,
	// encoding, or gas accounting. Only the native staking destination matters.
	candidate := *selp
	if container, ok := candidate.Envelope.(action.TxContainer); ok {
		to, _ := candidate.Destination()
		feature, enabled := protocol.GetFeatureCtx(ctx)
		if !enabled || !feature.EnableBucketGuard || to != address.StakingProtocolAddr {
			return nil
		}
		if err := container.Unfold(&candidate, ctx, func(_ context.Context, to *common.Address) (bool, bool, bool, error) {
			stakingAddr, err := address.FromString(address.StakingProtocolAddr)
			return true, to != nil && *to == common.BytesToAddress(stakingAddr.Bytes()), false, err
		}); err != nil {
			return err
		}
	}
	switch candidate.Action().(type) {
	case *action.BindBucketGuard, *action.GuardedBucketOp, *action.VaultAction:
		if !protocol.MustGetFeatureCtx(ctx).EnableBucketGuard {
			return nil
		}
		// Reject a bad outer ECDSA signature before spending SLH-DSA CPU.
		if err := selp.VerifySignature(); err != nil {
			return err
		}
		if v.chainContext != nil {
			ctx, err = v.chainContext(ctx)
			if err != nil {
				return err
			}
		}
		blkCtx := protocol.MustGetBlockCtx(ctx)
		blkCtx.BlockTimeStamp = protocol.MustGetBlockchainCtx(ctx).Tip.Timestamp.Add(genesis.MustExtractGenesisContext(ctx).BlockInterval)
		ctx = protocol.WithBlockCtx(ctx, blkCtx)
		ctx, err = withActionCtx(ctx, &candidate)
		if err != nil {
			return err
		}
		for _, p := range v.reg.All() {
			if checker, ok := p.(interface {
				PreflightBucketGuard(context.Context, action.Envelope, protocol.StateReader) error
			}); ok {
				return checker.PreflightBucketGuard(ctx, candidate.Envelope, v.sr)
			}
		}
		return errors.New("staking preflight is unavailable")
	}

	return nil
}

// withActionCtx mirrors the ActionCtx the block producer builds in
// state/factory. The protocols' validation rules read it, so an action has to
// be judged against the same values here as it would be at proposal time --
// otherwise the pool and the proposer disagree about what is admissible.
func withActionCtx(ctx context.Context, selp *action.SealedEnvelope) (context.Context, error) {
	caller := selp.SenderAddress()
	if caller == nil {
		return nil, errors.New("failed to get sender address")
	}
	actionCtx := protocol.ActionCtx{
		Caller:   caller,
		GasPrice: selp.GasPrice(),
		Nonce:    selp.Nonce(),
	}
	var err error
	if actionCtx.ActionHash, err = selp.Hash(); err != nil {
		return nil, err
	}
	if actionCtx.IntrinsicGas, err = selp.IntrinsicGas(); err != nil {
		return nil, err
	}
	return protocol.WithActionCtx(ctx, actionCtx), nil
}
