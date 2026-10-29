package bucketguard

import (
	"fmt"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

const addressLen = 20

// Validate performs the stateless checks of BindBucketGuard (IIP §5). Errors
// wrap ErrMalformed.
func (m *BindBucketGuard) Validate(p Params) error {
	if err := validateKey(p, m.AlgID, m.PublicKey, m.AcceptSig); err != nil {
		return err
	}
	if size := m.CanonicalSize(); size > p.MaxBindSize {
		return malformed("canonical size %d exceeds %d", size, p.MaxBindSize)
	}
	return nil
}

// Validate performs the stateless checks of GuardedBucketOp (IIP §9.1) that
// belong to the guard: sizes, deadline, and which fields each op carries.
// Errors wrap ErrMalformed.
//
// The staking business rules of the underlying operation (restake duration
// bound, candidate-name format) are not repeated here; the staking protocol
// must apply its existing checks to the op, as it does for the legacy actions.
func (m *GuardedBucketOp) Validate(p Params) error {
	if m.Deadline == 0 {
		return malformed("deadline must be > 0")
	}
	if !p.acceptsSigSize(len(m.Sig)) {
		return malformed("CALL signature has invalid size %d", len(m.Sig))
	}
	if len(m.Sponsor) != 0 && len(m.Sponsor) != addressLen {
		return malformed("sponsor must be empty or %d bytes, got %d", addressLen, len(m.Sponsor))
	}
	limit := p.MaxOpSize
	if m.installsKey() {
		limit = p.MaxOpWithNewGuardSize
	}
	if size := m.CanonicalSize(); size > limit {
		return malformed("canonical size %d exceeds %d", size, limit)
	}
	return m.validateFields(p)
}

// field presence per op; true means required, absent means must be empty.
type fieldSet struct {
	recipient, newOwner, candidate, newGuard, clearGuard, restake bool
}

func (m *GuardedBucketOp) validateFields(p Params) error {
	var want fieldSet
	switch m.Op {
	case OpUnstake:
	case OpWithdraw:
		want.recipient = true
	case OpTransfer:
		want.newOwner = true
		if m.installsKey() == m.ClearGuard {
			return malformed("TRANSFER requires exactly one of newGuard or clearGuard")
		}
		want.newGuard, want.clearGuard = m.installsKey(), m.ClearGuard
	case OpChangeCandidate:
		want.candidate = true
	case OpRestake:
		want.restake = true
	case OpRotate:
		want.newGuard = true
	default:
		return malformed("unknown op %d", uint32(m.Op))
	}
	if err := checkAddr(m.Op, "recipient", m.Recipient, want.recipient); err != nil {
		return err
	}
	if err := checkAddr(m.Op, "newOwner", m.NewOwner, want.newOwner); err != nil {
		return err
	}
	if err := checkAddr(m.Op, "expectedCandidateID", m.ExpectedCandidateID, want.candidate); err != nil {
		return err
	}
	if want.candidate != (m.CandidateName != "") {
		return unexpected(m.Op, "candidateName", want.candidate)
	}
	if !want.restake && (m.Duration != 0 || m.AutoStake) {
		return unexpected(m.Op, "duration/autoStake", false)
	}
	if want.newGuard != m.installsKey() {
		return unexpected(m.Op, "newGuard", want.newGuard)
	}
	if want.clearGuard != m.ClearGuard {
		return unexpected(m.Op, "clearGuard", want.clearGuard)
	}
	if m.installsKey() {
		return validateKey(p, m.NewGuard.AlgID, m.NewGuard.PublicKey, m.NewGuard.AcceptSig)
	}
	return nil
}

func checkAddr(op Op, name string, v []byte, required bool) error {
	switch {
	case required && len(v) != addressLen:
		return malformed("%s %s must be %d bytes, got %d", op, name, addressLen, len(v))
	case !required && len(v) != 0:
		return unexpected(op, name, false)
	}
	return nil
}

func validateKey(p Params, alg pqcrypto.AlgID, publicKey, acceptSig []byte) error {
	if !p.accepts(alg) {
		return malformed("unsupported algorithm %s", alg)
	}
	if len(publicKey) != alg.PublicKeySize() {
		return malformed("%s public key must be %d bytes, got %d", alg, alg.PublicKeySize(), len(publicKey))
	}
	if len(acceptSig) != alg.SignatureSize() {
		return malformed("%s accept signature must be %d bytes, got %d", alg, alg.SignatureSize(), len(acceptSig))
	}
	return nil
}

func malformed(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrMalformed}, args...)...)
}

func unexpected(op Op, field string, required bool) error {
	if required {
		return malformed("%s requires %s", op, field)
	}
	return malformed("%s must not carry %s", op, field)
}
