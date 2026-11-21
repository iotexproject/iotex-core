// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// An action whose amount field is absent on the wire must decode to a zero
// amount and be rejected by SanityCheck.
func TestEmptyStakedAmountDecodesAsZero(t *testing.T) {
	sk := identityset.PrivateKey(27)

	blsPub := make([]byte, 48)
	blsPub[0] = 0x01

	for _, v := range []struct {
		name          string
		wantSanityErr bool
		core          *iotextypes.ActionCore
	}{
		{
			"CreateStake",
			true,
			&iotextypes.ActionCore{
				Version:  1,
				Nonce:    1,
				GasLimit: 1000000,
				GasPrice: "1000000000000",
				Action: &iotextypes.ActionCore_StakeCreate{
					StakeCreate: &iotextypes.StakeCreate{
						CandidateName:  "candidate",
						StakedAmount:   "", // absent on the wire
						StakedDuration: 1,
					},
				},
			},
		},
		{
			"CandidateRegister",
			false,
			&iotextypes.ActionCore{
				Version:  1,
				Nonce:    1,
				GasLimit: 1000000,
				GasPrice: "1000000000000",
				Action: &iotextypes.ActionCore_CandidateRegister{
					CandidateRegister: &iotextypes.CandidateRegister{
						Candidate: &iotextypes.CandidateBasicInfo{
							Name:            "candidate",
							OperatorAddress: identityset.Address(28).String(),
							RewardAddress:   identityset.Address(29).String(),
						},
						StakedAmount:   "", // absent on the wire
						StakedDuration: 1,
						OwnerAddress:   identityset.Address(27).String(),
					},
				},
			},
		},
		{
			"CandidateRegisterWithBLS",
			false,
			&iotextypes.ActionCore{
				Version:  1,
				Nonce:    1,
				GasLimit: 1000000,
				GasPrice: "1000000000000",
				Action: &iotextypes.ActionCore_CandidateRegister{
					CandidateRegister: &iotextypes.CandidateRegister{
						Candidate: &iotextypes.CandidateBasicInfo{
							Name:            "candidate",
							OperatorAddress: identityset.Address(28).String(),
							RewardAddress:   identityset.Address(29).String(),
							BlsPubKey:       blsPub,
						},
						StakedAmount:   "", // absent on the wire
						StakedDuration: 1,
						OwnerAddress:   identityset.Address(27).String(),
					},
				},
			},
		},
	} {
		t.Run(v.name, func(t *testing.T) {
			r := require.New(t)
			// decode through the production wire path
			unsigned := &iotextypes.Action{
				Core:         v.core,
				SenderPubKey: sk.PublicKey().Bytes(),
				Signature:    make([]byte, 65),
				Encoding:     iotextypes.Encoding_IOTEX_PROTOBUF,
			}
			decoded, err := (&Deserializer{}).ActionToSealedEnvelope(unsigned)
			r.NoError(err)

			// sign it and confirm the action is rejected by validation
			signed, err := Sign(decoded.Envelope, sk)
			r.NoError(err)
			selp, err := (&Deserializer{}).ActionToSealedEnvelope(signed.Proto())
			r.NoError(err)
			r.NoError(selp.VerifySignature())
			// SanityCheck must reach a verdict on the zero amount. CreateStake
			// rejects a zero amount outright; for CandidateRegister a zero
			// amount is not a SanityCheck error (it was legal before Tsunami,
			// see the CandidateRegisterMustWithStake gate in
			// action/protocol/staking) and is rejected one layer up by
			// validateCandidateRegister -- do not tighten SanityCheck here, it
			// is also the historical-block decode path.
			r.NotPanics(func() {
				err := selp.Envelope.SanityCheck()
				if v.wantSanityErr {
					r.ErrorIs(err, ErrInvalidAmount)
				} else {
					r.NoError(err)
				}
			})

			// the amount decodes to zero, never to nil
			amount := decoded.Envelope.Action().(amountForCost).Amount()
			r.NotNil(amount)
			r.Zero(amount.Sign())
		})
	}
}

// After the fix an absent StakedAmount re-serializes as "0", so the action hash
// is computed over the "0" form. A signature produced over the absent form no
// longer matches and the action is rejected at signature verification, before
// it reaches any validator. Nothing legitimate signs the absent form: every
// typed constructor stringifies the amount, so a zero self-stake registration
// is always sent as an explicit "0".
func TestEmptyStakedAmountSignatureNoLongerVerifies(t *testing.T) {
	r := require.New(t)
	sk := identityset.PrivateKey(27)

	core := &iotextypes.ActionCore{
		Version:  1,
		Nonce:    1,
		GasLimit: 1000000,
		GasPrice: "1000000000000",
		Action: &iotextypes.ActionCore_StakeCreate{
			StakeCreate: &iotextypes.StakeCreate{
				CandidateName:  "candidate",
				StakedAmount:   "",
				StakedDuration: 1,
			},
		},
	}
	// sign the absent form
	raw, err := proto.Marshal(core)
	r.NoError(err)
	h := hash.Hash256b(raw)
	sig, err := sk.Sign(h[:])
	r.NoError(err)

	selp, err := (&Deserializer{}).ActionToSealedEnvelope(&iotextypes.Action{
		Core:         core,
		SenderPubKey: sk.PublicKey().Bytes(),
		Signature:    sig,
		Encoding:     iotextypes.Encoding_IOTEX_PROTOBUF,
	})
	r.NoError(err)
	r.ErrorIs(selp.VerifySignature(), ErrInvalidSender)
}
