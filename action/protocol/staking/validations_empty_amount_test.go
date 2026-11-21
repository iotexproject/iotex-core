// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// decodeEmptyAmountRegister decodes a CandidateRegister whose StakedAmount is
// absent on the wire, through the production deserializer.
func decodeRegisterWithAmount(t *testing.T, amount string) *action.CandidateRegister {
	t.Helper()
	sk := identityset.PrivateKey(27)
	pb := &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Version:  1,
			Nonce:    1,
			GasLimit: 1000000,
			GasPrice: "1000000000000",
			Action: &iotextypes.ActionCore_CandidateRegister{
				CandidateRegister: &iotextypes.CandidateRegister{
					Candidate: &iotextypes.CandidateBasicInfo{
						Name:            "cand",
						OperatorAddress: identityset.Address(2).String(),
						RewardAddress:   identityset.Address(3).String(),
					},
					StakedAmount:   amount,
					StakedDuration: 1,
					OwnerAddress:   identityset.Address(1).String(),
				},
			},
		},
		SenderPubKey: sk.PublicKey().Bytes(),
		Signature:    make([]byte, 65),
		Encoding:     iotextypes.Encoding_IOTEX_PROTOBUF,
	}
	selp, err := (&action.Deserializer{}).ActionToSealedEnvelope(pb)
	require.NoError(t, err)
	cr, ok := selp.Envelope.Action().(*action.CandidateRegister)
	require.True(t, ok)
	return cr
}

// An absent StakedAmount decodes to zero (never nil) and is then indistinguishable
// from an explicit "0" on the wire -- which post-Tsunami is a legitimate
// zero-self-stake candidate registration (the CandidateRegisterMustWithStake
// gate is !IsTsunami, so the zero-amount exemption is what is live on MainNet).
// The fix therefore adds no new action shape: it makes an absent field an alias
// for "0", the same convention Transfer/Execution/DepositToStake already use.
//
// SanityCheck must keep tolerating a zero amount either way: LoadProto is also
// the historical-block decode path, and both eras are represented in history.
func TestValidateCandidateRegisterEmptyAmount(t *testing.T) {
	r := require.New(t)
	p := &Protocol{}
	p.config.RegistrationConsts.MinSelfStake = big.NewInt(1)

	ctxAt := func(tsunami uint64) context.Context {
		g := genesis.TestDefault()
		g.XinguBlockHeight = math.MaxUint64
		g.XinguBetaBlockHeight = math.MaxUint64
		g.YapBlockHeight = math.MaxUint64
		g.YapBetaBlockHeight = math.MaxUint64
		g.ZanzibarBlockHeight = math.MaxUint64
		g.ZanzibarBetaBlockHeight = math.MaxUint64
		g.TsunamiBlockHeight = tsunami
		ctx := genesis.WithGenesisContext(context.Background(), g)
		return protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: 1}))
	}

	empty := decodeRegisterWithAmount(t, "")
	zero := decodeRegisterWithAmount(t, "0")

	// absent decodes to zero, never to nil, and matches an explicit "0"
	r.NotNil(empty.Amount())
	r.Zero(empty.Amount().Sign())
	r.Equal(zero.Amount(), empty.Amount())
	r.NoError(empty.SanityCheck())

	for _, era := range []struct {
		name    string
		tsunami uint64
		wantErr bool
	}{
		{"post-Tsunami (live): zero self-stake is legal", 0, false},
		{"pre-Tsunami: registration must carry a stake", math.MaxUint64, true},
	} {
		t.Run(era.name, func(t *testing.T) {
			ctx := ctxAt(era.tsunami)
			errEmpty := p.validateCandidateRegister(ctx, empty)
			errZero := p.validateCandidateRegister(ctx, zero)
			if era.wantErr {
				r.ErrorIs(errEmpty, action.ErrInvalidAmount)
			} else {
				r.NoError(errEmpty)
			}
			// the whole point: absent and "0" are now the same action
			r.Equal(errZero == nil, errEmpty == nil)
		})
	}
}
