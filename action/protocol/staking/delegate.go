// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

/*
 * keys in namespace factory.DelegateNameSpace are prefixed with 1-byte tag
 * to indicate the type of stored object. This enables to quickly scan the
 * entire bucket and filter out objects of a certain type
 */
const (
	inactiveDel = byte(iota)
	activeDel   = inactiveDel + 1
)

var (
	// ErrInvalidAmount indicates an invalid staking amount
	ErrInvalidAmount = errors.New("invalid staking amount")
)

type (
	// CandName is the 12-byte delegate name
	CandName [12]byte

	// Delegate represents the delegate
	Delegate struct {
		Owner         address.Address
		Address       string
		RewardAddress string
		CanName       CandName
		Votes         *big.Int
		SelfStake     *big.Int
		Active        bool
	}

	// DelegateList is a list of delegates which is sortable
	DelegateList []*Delegate
)

// NewDelegate creates a delegate
func NewDelegate(owner, addr, reward, name, amount, self string) (*Delegate, error) {
	if len(name) == 0 || len(name) != 12 {
		return nil, errors.Errorf("invalid candidate name length %d, expecting 12", len(name))
	}

	vote, ok := big.NewInt(0).SetString(amount, 10)
	if !ok {
		return nil, ErrInvalidAmount
	}

	if vote.Sign() <= 0 {
		return nil, ErrInvalidAmount
	}

	selfStake, ok := big.NewInt(0).SetString(self, 10)
	if !ok {
		return nil, ErrInvalidAmount
	}

	if selfStake.Sign() <= 0 {
		return nil, ErrInvalidAmount
	}

	ownerAddr, err := address.FromString(owner)
	if err != nil {
		return nil, err
	}

	if _, err := address.FromString(addr); err != nil {
		return nil, err
	}

	if _, err := address.FromString(reward); err != nil {
		return nil, err
	}

	d := Delegate{
		Owner:         ownerAddr,
		Address:       addr,
		RewardAddress: reward,
		CanName:       ToCandName([]byte(name)),
		Votes:         vote,
		SelfStake:     selfStake,
		Active:        true,
	}
	return &d, nil
}

// ToCandName converts byte slice to CandName
func ToCandName(b []byte) CandName {
	var c CandName
	if len(b) > 12 {
		b = b[len(b)-12:]
	}
	copy(c[12-len(b):], b)
	return c
}

// Validate checks the validity of the delegate
func (d *Delegate) Validate() error {
	if _, err := address.FromString(d.Address); err != nil {
		return errors.Wrap(err, "invalid operator address")
	}

	if _, err := address.FromString(d.RewardAddress); err != nil {
		return errors.Wrap(err, "invalid reward address")
	}

	if d.Votes.Sign() <= 0 {
		return ErrInvalidAmount
	}
	return nil
}

// AddVote adds vote
func (d *Delegate) AddVote(amount *big.Int) error {
	if amount.Sign() < 0 {
		return ErrInvalidAmount
	}
	d.Votes.Add(d.Votes, amount)
	return nil
}

// SubVote subtracts vote
func (d *Delegate) SubVote(amount *big.Int) error {
	if amount.Sign() < 0 {
		return ErrInvalidAmount
	}

	if d.Votes.Cmp(amount) == -1 {
		return ErrInvalidAmount
	}
	d.Votes.Sub(d.Votes, amount)
	return nil
}

// Serialize serializes delegate to bytes
func (d *Delegate) Serialize() ([]byte, error) {
	return proto.Marshal(d.toProto())
}

// Deserialize deserializes bytes to delegate
func (d *Delegate) Deserialize(buf []byte) error {
	pb := &stakingpb.Delegate{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal delegate")
	}

	ownerAddr, err := address.FromString(pb.Owner)
	if err != nil {
		return err
	}

	d.Owner = ownerAddr
	d.Address = pb.Address
	d.RewardAddress = pb.RewardAddress
	d.CanName = ToCandName(pb.CanName)
	d.Active = pb.Active

	if len(pb.Votes) > 0 {
		d.Votes = new(big.Int).SetBytes(pb.Votes)
	}

	if len(pb.SelfStake) > 0 {
		d.SelfStake = new(big.Int).SetBytes(pb.SelfStake)
	}
	return nil
}

func (d *Delegate) toProto() *stakingpb.Delegate {
	name := make([]byte, len(d.CanName))
	copy(name, d.CanName[:])
	return &stakingpb.Delegate{
		Owner:         d.Owner.String(),
		Address:       d.Address,
		RewardAddress: d.RewardAddress,
		CanName:       name,
		Votes:         d.Votes.Bytes(),
		SelfStake:     d.SelfStake.Bytes(),
		Active:        d.Active,
	}
}

func fromProto(pb *stakingpb.Delegate) *Delegate {
	ownerAddr, err := address.FromString(pb.Owner)
	if err != nil {
		return nil
	}

	d := Delegate{
		Owner:         ownerAddr,
		Address:       pb.Address,
		RewardAddress: pb.RewardAddress,
		CanName:       ToCandName(pb.CanName),
		Active:        pb.Active,
	}

	if len(pb.Votes) > 0 {
		d.Votes = new(big.Int).SetBytes(pb.Votes)
	}

	if len(pb.SelfStake) > 0 {
		d.SelfStake = new(big.Int).SetBytes(pb.SelfStake)
	}
	return &d
}

func (l DelegateList) Len() int      { return len(l) }
func (l DelegateList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l DelegateList) Less(i, j int) bool {
	if res := l[i].Votes.Cmp(l[j].Votes); res != 0 {
		return res == 1
	}
	return strings.Compare(l[i].Owner.String(), l[j].Owner.String()) == 1
}

func (l DelegateList) toProto() *stakingpb.Delegates {
	delegatePb := make([]*stakingpb.Delegate, len(l))
	for i, del := range l {
		delegatePb[i] = del.toProto()
	}
	return &stakingpb.Delegates{Delegates: delegatePb}
}

// Deserialize deserializes bytes to list of delegates
func (l *DelegateList) Deserialize(buf []byte) error {
	pb := &stakingpb.Delegates{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal delegate list")
	}

	*l = (*l)[:0]
	for _, v := range pb.Delegates {
		*l = append(*l, fromProto(v))
	}
	return nil
}

func stakingGetDelegate(sr protocol.StateReader, name address.Address) (*Delegate, error) {
	key := make([]byte, len(name.Bytes())+1)
	key[0] = activeDel
	copy(key[1:], name.Bytes())

	var d Delegate
	_, err := sr.State(&d, protocol.NamespaceOption(factory.DelegateNameSpace), protocol.KeyOption(key))
	if err == nil {
		return &d, nil
	}

	if errors.Cause(err) != state.ErrStateNotExist {
		return nil, err
	}

	key[0] = inactiveDel
	if _, err = sr.State(&d, protocol.NamespaceOption(factory.DelegateNameSpace), protocol.KeyOption(key)); err != nil {
		return nil, err
	}
	return &d, nil
}

func stakingPutDelegate(sm protocol.StateManager, name address.Address, d *Delegate) error {
	key := make([]byte, len(name.Bytes())+1)
	if d.Active {
		key[0] = activeDel
	} else {
		key[0] = inactiveDel
	}
	copy(key[1:], name.Bytes())

	_, err := sm.PutState(d, protocol.NamespaceOption(factory.DelegateNameSpace), protocol.KeyOption(key))
	return err
}

func stakingDelDelegate(sm protocol.StateManager, name address.Address, active bool) error {
	key := make([]byte, len(name.Bytes())+1)
	if active {
		key[0] = activeDel
	} else {
		key[0] = inactiveDel
	}
	copy(key[1:], name.Bytes())

	_, err := sm.DelState(protocol.NamespaceOption(factory.DelegateNameSpace), protocol.KeyOption(key))
	return err
}
