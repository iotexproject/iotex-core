// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package actpool

import (
	"context"
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// rejectingProtocol stands in for a protocol whose feature gate makes an action
// unacceptable at the current height.
type rejectingProtocol struct {
	err    error
	sawCtx bool
}

func (p *rejectingProtocol) Name() string { return "rejecting" }
func (p *rejectingProtocol) Handle(context.Context, action.Envelope, protocol.StateManager) (*action.Receipt, error) {
	return nil, nil
}
func (p *rejectingProtocol) ReadState(context.Context, protocol.StateReader, []byte, ...[]byte) ([]byte, uint64, error) {
	return nil, 0, nil
}
func (p *rejectingProtocol) Register(r *protocol.Registry) error { return r.Register(p.Name(), p) }
func (p *rejectingProtocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(p.Name(), p)
}

func (p *rejectingProtocol) Validate(ctx context.Context, _ action.Envelope, _ protocol.StateReader) error {
	// The protocols' own rules read ActionCtx; assert the validator supplies it
	// rather than leaving them to panic on MustGetActionCtx.
	if _, ok := protocol.GetActionCtx(ctx); ok {
		p.sawCtx = true
	}
	return p.err
}

func testEnvelope(t *testing.T) *action.SealedEnvelope {
	t.Helper()
	tsf := action.NewTransfer(big.NewInt(1), identityset.Address(1).String(), nil)
	elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(100000).
		SetGasPrice(big.NewInt(1)).SetAction(tsf).Build()
	selp, err := action.Sign(elp, identityset.PrivateKey(0))
	require.NoError(t, err)
	return selp
}

// A protocol that rejects the action must make admission fail, so the sender
// gets an error from eth_sendRawTransaction instead of a nonce that silently
// stalls every later action from that account.
func TestProtocolValidatorRejectsAtAdmission(t *testing.T) {
	r := require.New(t)
	reg := protocol.NewRegistry()
	p := &rejectingProtocol{err: errors.New("feature not enabled yet")}
	r.NoError(p.Register(reg))

	v := NewProtocolValidator(reg, nil)
	err := v.Validate(context.Background(), testEnvelope(t))
	r.ErrorContains(err, "feature not enabled yet")
	r.True(p.sawCtx, "protocol validators must see an ActionCtx")
}

// The registry is read at validation time, not captured at construction: the
// action pool is built before any protocol is registered.
func TestProtocolValidatorResolvesRegistryLazily(t *testing.T) {
	r := require.New(t)
	reg := protocol.NewRegistry()

	// Built while the registry is still empty, exactly as chainservice does.
	v := NewProtocolValidator(reg, nil)
	selp := testEnvelope(t)
	r.NoError(v.Validate(context.Background(), selp))

	p := &rejectingProtocol{err: errors.New("registered later")}
	r.NoError(p.Register(reg))
	r.ErrorContains(v.Validate(context.Background(), selp), "registered later")
}

// System actions are proposer-produced and several carry no sender, so they
// must bypass the ActionCtx construction rather than error out.
func TestProtocolValidatorSkipsSystemActions(t *testing.T) {
	r := require.New(t)
	reg := protocol.NewRegistry()
	p := &rejectingProtocol{err: errors.New("should not be consulted")}
	r.NoError(p.Register(reg))

	gr := action.NewGrantReward(action.BlockReward, 1)
	elp := (&action.EnvelopeBuilder{}).SetNonce(0).SetGasLimit(0).SetAction(gr).Build()
	selp, err := action.Sign(elp, identityset.PrivateKey(0))
	r.NoError(err)
	r.True(action.IsSystemAction(selp))

	r.NoError(v(reg).Validate(context.Background(), selp))
	r.False(p.sawCtx)
}

func v(reg *protocol.Registry) action.SealedEnvelopeValidator {
	return NewProtocolValidator(reg, nil)
}

// WithPrivateValidator must land the validator on the Add-only list. Putting it
// on actionEnvelopeValidators would also run it in actPool.Validate, which
// block validation reaches through the bundle pool -- turning "included with a
// failure receipt" into "invalid block", a consensus change.
func TestPrivateValidatorIsNotOnTheBlockValidationPath(t *testing.T) {
	r := require.New(t)
	ap := &actPool{}
	r.NoError(WithPrivateValidator(NewProtocolValidator(protocol.NewRegistry(), nil))(ap))

	r.Len(ap.privateValidators, 1)
	r.Empty(ap.actionEnvelopeValidators,
		"protocol validation must not reach actPool.Validate, which block validation uses")

	r.Error(WithPrivateValidator(nil)(ap))
}
