// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

var _voterRewardDestinationKeyPrefix = []byte("vrd")

type voterRewardDestination struct {
	recipient     address.Address
	updatedHeight uint64
}

func (d voterRewardDestination) Serialize() ([]byte, error) {
	m := &rewardingpb.VoterRewardDestination{
		ExplicitlySet: d.recipient != nil,
		UpdatedHeight: d.updatedHeight,
	}
	if d.recipient != nil {
		m.Recipient = d.recipient.Bytes()
	}
	return proto.Marshal(m)
}

func (d *voterRewardDestination) Deserialize(data []byte) error {
	m := &rewardingpb.VoterRewardDestination{}
	if err := proto.Unmarshal(data, m); err != nil {
		return err
	}
	d.updatedHeight = m.GetUpdatedHeight()
	if len(m.GetRecipient()) == 0 {
		return errors.New("empty stored voter reward destination")
	}
	if len(m.GetRecipient()) != common.AddressLength {
		return errors.Errorf("invalid stored voter reward destination length %d", len(m.GetRecipient()))
	}
	if !m.GetExplicitlySet() {
		return errors.New("stored voter reward destination is not marked explicit")
	}
	if common.BytesToAddress(m.GetRecipient()) == (common.Address{}) {
		return errors.New("zero stored voter reward destination")
	}
	recipient, err := address.FromBytes(m.GetRecipient())
	if err != nil {
		return errors.Wrap(err, "invalid stored voter reward destination")
	}
	d.recipient = recipient
	return nil
}

func (d *voterRewardDestination) Encode() (systemcontracts.GenericValue, error) {
	data, err := d.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, err
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

func (d *voterRewardDestination) Decode(v systemcontracts.GenericValue) error {
	return d.Deserialize(v.PrimaryData)
}

func voterRewardDestinationKey(voter address.Address) []byte {
	key := make([]byte, 0, len(_voterRewardDestinationKeyPrefix)+len(voter.Bytes()))
	key = append(key, _voterRewardDestinationKeyPrefix...)
	return append(key, voter.Bytes()...)
}

// readVoterRewardDestination returns nil when the voter has no explicit
// override. Stored entries are always explicit; the voter itself is the
// effective default and is never persisted.
func (p *Protocol) readVoterRewardDestination(
	ctx context.Context,
	sr protocol.StateReader,
	voter address.Address,
) (*voterRewardDestination, error) {
	destination := &voterRewardDestination{}
	if _, err := p.state(ctx, sr, voterRewardDestinationKey(voter), destination); err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if destination.recipient == nil {
		return nil, errors.New("nil stored voter reward destination")
	}
	if bytes.Equal(destination.recipient.Bytes(), voter.Bytes()) {
		return nil, errors.New("stored voter reward destination equals voter")
	}
	return destination, nil
}

func (p *Protocol) effectiveVoterRewardDestination(
	ctx context.Context,
	sr protocol.StateReader,
	voter address.Address,
) (address.Address, error) {
	destination, err := p.readVoterRewardDestination(ctx, sr, voter)
	if err != nil {
		return nil, err
	}
	if destination == nil {
		return voter, nil
	}
	return destination.recipient, nil
}

func (p *Protocol) setVoterRewardDestination(
	ctx context.Context,
	sm protocol.StateManager,
	voter address.Address,
	recipientBytes []byte,
) (address.Address, address.Address, error) {
	oldRecipient, err := p.effectiveVoterRewardDestination(ctx, sm, voter)
	if err != nil {
		return nil, nil, err
	}

	var recipient address.Address
	if len(recipientBytes) > 0 {
		if len(recipientBytes) != common.AddressLength {
			return nil, nil, errors.Errorf("invalid voter reward destination length %d", len(recipientBytes))
		}
		recipient, err = address.FromBytes(recipientBytes)
		if err != nil {
			return nil, nil, errors.Wrap(err, "invalid voter reward destination")
		}
	}
	if recipient == nil || common.BytesToAddress(recipient.Bytes()) == (common.Address{}) ||
		bytes.Equal(recipient.Bytes(), voter.Bytes()) {
		if err := p.deleteState(ctx, sm, voterRewardDestinationKey(voter), &voterRewardDestination{}); err != nil {
			return nil, nil, err
		}
		return oldRecipient, voter, nil
	}

	if err := p.putState(ctx, sm, voterRewardDestinationKey(voter), &voterRewardDestination{
		recipient:     recipient,
		updatedHeight: protocol.MustGetBlockCtx(ctx).BlockHeight,
	}); err != nil {
		return nil, nil, err
	}
	return oldRecipient, recipient, nil
}
