package actpool

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/iotxaddress"
)

// ActionValidator is the interface of validating an action
type ActionValidator interface {
	Validate(action.Action) error
}

type transferValidator struct{ bc blockchain.Blockchain }

type voteValidator struct{ bc blockchain.Blockchain }

type execValidator struct{ bc blockchain.Blockchain }

// NewTransferValidator creates a new transfer validator
func NewTransferValidator(bc blockchain.Blockchain) ActionValidator {
	return &transferValidator{bc}
}

// NewVoteValidator creates a new vote validator
func NewVoteValidator(bc blockchain.Blockchain) ActionValidator {
	return &voteValidator{bc}
}

// NewExecValidator creates a new execution validator
func NewExecValidator(bc blockchain.Blockchain) ActionValidator {
	return &execValidator{bc}
}

// Validate validates a transfer
func (v *transferValidator) Validate(act action.Action) error {
	tsf, ok := act.(*action.Transfer)
	if !ok {
		return nil
	}
	// Reject coinbase transfer
	if tsf.IsCoinbase() {
		return errors.Wrapf(ErrTransfer, "coinbase transfer")
	}
	// Reject oversized transfer
	if tsf.TotalSize() > TransferSizeLimit {
		return errors.Wrapf(ErrActPool, "oversized data")
	}
	// Reject over-gassed transfer
	if tsf.GasLimit() > blockchain.GasLimit {
		return errors.Wrapf(ErrGasHigherThanLimit, "gas is higher than gas limit")
	}
	// Reject transfer of insufficient gas limit
	intrinsicGas, err := tsf.IntrinsicGas()
	if intrinsicGas > tsf.GasLimit() || err != nil {
		return errors.Wrapf(ErrInsufficientGas, "insufficient gas for transfer")
	}
	// Reject transfer of negative amount
	if tsf.Amount().Sign() < 0 {
		return errors.Wrapf(ErrBalance, "negative value")
	}

	// check if sender's address is valid
	if _, err := iotxaddress.GetPubkeyHash(tsf.Sender()); err != nil {
		return errors.Wrapf(err, "error when validating sender's address %s", tsf.Sender())
	}
	// check if recipient's address is valid
	if _, err := iotxaddress.GetPubkeyHash(tsf.Recipient()); err != nil {
		return errors.Wrapf(err, "error when validating recipient's address %s", tsf.Recipient())
	}

	// Verify transfer using sender's public key
	if err := action.Verify(tsf); err != nil {
		return errors.Wrapf(err, "failed to verify Transfer signature")
	}
	// Reject transfer if nonce is too low
	confirmedNonce, err := v.bc.Nonce(tsf.Sender())
	if err != nil {
		return errors.Wrapf(err, "invalid nonce value")
	}
	pendingNonce := confirmedNonce + 1
	if pendingNonce > tsf.Nonce() {
		return errors.Wrapf(ErrNonce, "nonce too low")
	}
	return nil
}

// Validate validates a vote
func (v *voteValidator) Validate(act action.Action) error {
	vote, ok := act.(*action.Vote)
	if !ok {
		return nil
	}
	// Reject oversized vote
	if vote.TotalSize() > VoteSizeLimit {
		return errors.Wrapf(ErrActPool, "oversized data")
	}
	// Reject over-gassed transfer
	if vote.GasLimit() > blockchain.GasLimit {
		return errors.Wrapf(ErrGasHigherThanLimit, "gas is higher than gas limit")
	}
	// Reject transfer of insufficient gas limit
	intrinsicGas, err := vote.IntrinsicGas()
	if intrinsicGas > vote.GasLimit() || err != nil {
		return errors.Wrapf(ErrInsufficientGas, "insufficient gas for vote")
	}
	// check if voter's address is valid
	if _, err := iotxaddress.GetPubkeyHash(vote.Voter()); err != nil {
		return errors.Wrapf(err, "error when validating voter's address %s", vote.Voter())
	}
	// check if votee's address is valid
	if vote.Votee() != action.EmptyAddress {
		if _, err := iotxaddress.GetPubkeyHash(vote.Votee()); err != nil {
			return errors.Wrapf(err, "error when validating votee's address %s", vote.Votee())
		}
	}

	// Verify vote using voter's public key
	if err := action.Verify(vote); err != nil {
		return errors.Wrapf(err, "failed to verify vote signature")
	}

	// Reject vote if nonce is too low
	confirmedNonce, err := v.bc.Nonce(vote.Voter())
	if err != nil {
		return errors.Wrapf(err, "invalid nonce value")
	}

	if vote.Votee() != "" {
		// Reject vote if votee is not a candidate
		voteeState, err := v.bc.StateByAddr(vote.Votee())
		if err != nil {
			return errors.Wrapf(err, "cannot find votee's state: %s", vote.Votee())
		}
		if vote.Voter() != vote.Votee() && !voteeState.IsCandidate {
			return errors.Wrapf(ErrVotee, "votee has not self-nominated: %s", vote.Votee())
		}
	}

	pendingNonce := confirmedNonce + 1
	if pendingNonce > vote.Nonce() {
		return errors.Wrapf(ErrNonce, "nonce too low")
	}
	return nil
}

// Validate validates an execution
func (v *execValidator) Validate(act action.Action) error {
	exec, ok := act.(*action.Execution)
	if !ok {
		return nil
	}
	// Reject oversized exeuction
	if exec.TotalSize() > ExecutionSizeLimit {
		return errors.Wrapf(ErrActPool, "oversized data")
	}
	// Reject over-gassed execution
	if exec.GasLimit() > blockchain.GasLimit {
		return errors.Wrapf(ErrGasHigherThanLimit, "gas is higher than gas limit")
	}
	// Reject execution of insufficient gas limit
	intrinsicGas, err := exec.IntrinsicGas()
	if intrinsicGas > exec.GasLimit() || err != nil {
		return errors.Wrapf(ErrInsufficientGas, "insufficient gas for execution")
	}
	// Reject execution of negative amount
	if exec.Amount().Sign() < 0 {
		return errors.Wrapf(ErrBalance, "negative value")
	}

	// check if executor's address is valid
	if _, err := iotxaddress.GetPubkeyHash(exec.Executor()); err != nil {
		return errors.Wrapf(err, "error when validating executor's address %s", exec.Executor())
	}
	// check if contract's address is valid
	if exec.Contract() != action.EmptyAddress {
		if _, err := iotxaddress.GetPubkeyHash(exec.Contract()); err != nil {
			return errors.Wrapf(err, "error when validating contract's address %s", exec.Contract())
		}
	}

	// Verify transfer using executor's public key
	if err := action.Verify(exec); err != nil {
		return errors.Wrapf(err, "failed to verify Execution signature")
	}
	// Reject transfer if nonce is too low
	confirmedNonce, err := v.bc.Nonce(exec.Executor())
	if err != nil {
		return errors.Wrapf(err, "invalid nonce value")
	}
	pendingNonce := confirmedNonce + 1
	if pendingNonce > exec.Nonce() {
		return errors.Wrapf(ErrNonce, "nonce too low")
	}
	return nil
}
