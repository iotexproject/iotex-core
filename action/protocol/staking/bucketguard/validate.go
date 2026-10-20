package bucketguard

import "fmt"

// Validate performs stateless validation for BindBucketGuard.
func (m *BindBucketGuard) Validate() error {
	if m.AlgID != AlgSLHDSA192s {
		return fmt.Errorf("unsupported algID 0x%x", m.AlgID)
	}
	if len(m.PublicKey) != PubKeySize {
		return fmt.Errorf("public key must be %d bytes, got %d", PubKeySize, len(m.PublicKey))
	}
	if len(m.AcceptSig) != SigSize {
		return fmt.Errorf("accept signature must be %d bytes, got %d", SigSize, len(m.AcceptSig))
	}
	if size := m.CanonicalSize(); size > MaxBindSize {
		return fmt.Errorf("canonical size %d exceeds %d", size, MaxBindSize)
	}
	return nil
}

// Validate performs stateless validation for GuardedBucketOp.
func (m *GuardedBucketOp) Validate() error {
	if m.Deadline == 0 {
		return fmt.Errorf("deadline must be > 0")
	}
	if len(m.Sig) != SigSize {
		return fmt.Errorf("CALL signature must be %d bytes, got %d", SigSize, len(m.Sig))
	}
	limit := MaxOpSize
	if m.NewGuard != nil {
		limit = MaxOpWithNewGuardSize
	}
	if size := m.CanonicalSize(); size > limit {
		return fmt.Errorf("canonical size %d exceeds %d", size, limit)
	}
	return m.validateOpFields()
}

func (m *GuardedBucketOp) validateOpFields() error {
	switch m.Op {
	case OpUnstake:
		if m.hasAnyOpField() {
			return fmt.Errorf("UNSTAKE must not carry op-specific fields")
		}
	case OpWithdraw:
		if len(m.Recipient) != len(Address{}) {
			return fmt.Errorf("WITHDRAW recipient must be 20 bytes")
		}
		if len(m.NewOwner) != 0 || m.CandidateName != "" || len(m.ExpectedCandidateID) != 0 ||
			m.Duration != 0 || m.AutoStake || m.NewGuard != nil || m.ClearGuard {
			return fmt.Errorf("WITHDRAW carries unused fields")
		}
	case OpTransfer:
		if len(m.NewOwner) != len(Address{}) {
			return fmt.Errorf("TRANSFER newOwner must be 20 bytes")
		}
		if (m.NewGuard == nil) == !m.ClearGuard {
			return fmt.Errorf("TRANSFER requires exactly one of newGuard or clearGuard")
		}
		if m.NewGuard != nil {
			if err := m.NewGuard.validate(); err != nil {
				return err
			}
		}
		if len(m.Recipient) != 0 || m.CandidateName != "" || len(m.ExpectedCandidateID) != 0 ||
			m.Duration != 0 || m.AutoStake {
			return fmt.Errorf("TRANSFER carries unused fields")
		}
	case OpChangeCandidate:
		if err := validateCandidateName(m.CandidateName); err != nil {
			return err
		}
		if len(m.ExpectedCandidateID) != len(Address{}) {
			return fmt.Errorf("CHANGE_CANDIDATE expectedCandidateID must be 20 bytes")
		}
		if len(m.Recipient) != 0 || len(m.NewOwner) != 0 || m.Duration != 0 || m.AutoStake ||
			m.NewGuard != nil || m.ClearGuard {
			return fmt.Errorf("CHANGE_CANDIDATE carries unused fields")
		}
	case OpRestake:
		if m.Duration == 0 || m.Duration > MaxRestakeDurationDays {
			return fmt.Errorf("RESTAKE duration must be in 1..%d", MaxRestakeDurationDays)
		}
		if len(m.Recipient) != 0 || len(m.NewOwner) != 0 || m.CandidateName != "" ||
			len(m.ExpectedCandidateID) != 0 || m.NewGuard != nil || m.ClearGuard {
			return fmt.Errorf("RESTAKE carries unused fields")
		}
	case OpRotate:
		if m.NewGuard == nil {
			return fmt.Errorf("ROTATE requires newGuard")
		}
		if err := m.NewGuard.validate(); err != nil {
			return err
		}
		if m.ClearGuard {
			return fmt.Errorf("ROTATE must not set clearGuard")
		}
		if len(m.Recipient) != 0 || len(m.NewOwner) != 0 || m.CandidateName != "" ||
			len(m.ExpectedCandidateID) != 0 || m.Duration != 0 || m.AutoStake {
			return fmt.Errorf("ROTATE carries unused fields")
		}
	default:
		return fmt.Errorf("unknown op %d", m.Op)
	}
	return nil
}

func (m *GuardedBucketOp) hasAnyOpField() bool {
	return len(m.Recipient) != 0 || len(m.NewOwner) != 0 || m.CandidateName != "" ||
		len(m.ExpectedCandidateID) != 0 || m.Duration != 0 || m.AutoStake ||
		m.NewGuard != nil || m.ClearGuard
}

func (n *NewGuard) validate() error {
	if n == nil {
		return fmt.Errorf("newGuard is required")
	}
	if n.AlgID != AlgSLHDSA192s {
		return fmt.Errorf("unsupported newGuard algID 0x%x", n.AlgID)
	}
	if len(n.PublicKey) != PubKeySize {
		return fmt.Errorf("newGuard public key must be %d bytes", PubKeySize)
	}
	if len(n.AcceptSig) != SigSize {
		return fmt.Errorf("newGuard accept signature must be %d bytes", SigSize)
	}
	return nil
}

// validateCandidateName mirrors action.IsValidCandidateName: 1..12 lowercase
// ASCII letters or digits.
func validateCandidateName(name string) error {
	if len(name) == 0 || len(name) > MaxCandidateNameLen {
		return fmt.Errorf("candidate name must be 1..%d bytes", MaxCandidateNameLen)
	}
	for _, c := range name {
		if !(('a' <= c && c <= 'z') || ('0' <= c && c <= '9')) {
			return fmt.Errorf("candidate name must be lowercase alphanumeric")
		}
	}
	return nil
}
