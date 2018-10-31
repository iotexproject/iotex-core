package transfer

import (
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/state"
)

// TransferSizeLimit is the maximum size of transfer allowed
const TransferSizeLimit = 32 * 1024

// Protocol defines the protocol of handling transfers
type Protocol struct{}

// NewProtocol instantiates the protocol of transfer
func NewProtocol() *Protocol { return &Protocol{} }

// Handle handles a transfer
func (p *Protocol) Handle(act action.Action, ws state.WorkingSet) error {
	tsf, ok := act.(*action.Transfer)
	if !ok {
		return nil
	}
	if tsf.IsContract() {
		return nil
	}
	if !tsf.IsCoinbase() {
		// check sender
		sender, err := ws.LoadOrCreateAccountState(tsf.Sender(), big.NewInt(0))
		if err != nil {
			return errors.Wrapf(err, "failed to load or create the account of sender %s", tsf.Sender())
		}
		if tsf.Amount().Cmp(sender.Balance) == 1 {
			return errors.Wrapf(state.ErrNotEnoughBalance, "failed to verify the Balance of sender %s", tsf.Sender())
		}
		// update sender Balance
		if err := sender.SubBalance(tsf.Amount()); err != nil {
			return errors.Wrapf(err, "failed to update the Balance of sender %s", tsf.Sender())
		}
		// update sender Nonce
		if tsf.Nonce() > sender.Nonce {
			sender.Nonce = tsf.Nonce()
		}
		// Update sender votes
		if len(sender.Votee) > 0 && sender.Votee != tsf.Sender() {
			// sender already voted to a different person
			voteeOfSender, err := ws.LoadOrCreateAccountState(sender.Votee, big.NewInt(0))
			if err != nil {
				return errors.Wrapf(err, "failed to load or create the account of sender's votee %s", sender.Votee)
			}
			voteeOfSender.VotingWeight.Sub(voteeOfSender.VotingWeight, tsf.Amount())
		}
	}
	// check recipient
	recipient, err := ws.LoadOrCreateAccountState(tsf.Recipient(), big.NewInt(0))
	if err != nil {
		return errors.Wrapf(err, "failed to laod or create the account of recipient %s", tsf.Recipient())
	}
	if err := recipient.AddBalance(tsf.Amount()); err != nil {
		return errors.Wrapf(err, "failed to update the Balance of recipient %s", tsf.Recipient())
	}
	// Update recipient votes
	if len(recipient.Votee) > 0 && recipient.Votee != tsf.Recipient() {
		// recipient already voted to a different person
		voteeOfRecipient, err := ws.LoadOrCreateAccountState(recipient.Votee, big.NewInt(0))
		if err != nil {
			return errors.Wrapf(err, "failed to load or create the account of recipient's votee %s", recipient.Votee)
		}
		voteeOfRecipient.VotingWeight.Add(voteeOfRecipient.VotingWeight, tsf.Amount())
	}
	return nil
}

// Validate validates a transfer
func (p *Protocol) Validate(act action.Action) error {
	tsf, ok := act.(*action.Transfer)
	if !ok {
		return nil
	}
	// Reject coinbase transfer
	if tsf.IsCoinbase() {
		return errors.Wrap(action.ErrTransfer, "coinbase transfer")
	}
	// Reject oversized transfer
	if tsf.TotalSize() > TransferSizeLimit {
		return errors.Wrap(action.ErrActPool, "oversized data")
	}
	// Reject transfer of negative amount
	if tsf.Amount().Sign() < 0 {
		return errors.Wrap(action.ErrBalance, "negative value")
	}
	// check if recipient's address is valid
	if _, err := iotxaddress.GetPubkeyHash(tsf.Recipient()); err != nil {
		return errors.Wrapf(err, "error when validating recipient's address %s", tsf.Recipient())
	}
	return nil
}
