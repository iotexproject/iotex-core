package stakingindex

import (
	"math/big"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/state"
)

var (
	b1 = []*staking.VoteBucket{
		&staking.VoteBucket{
			Index:            0,
			Candidate:        &address.AddrV1{},
			Owner:            &address.AddrV1{},
			StakedAmount:     &big.Int{},
			StakedDuration:   time.Duration(0),
			CreateTime:       time.Time{},
			StakeStartTime:   time.Time{},
			UnstakeStartTime: time.Time{},
		},
		&staking.VoteBucket{
			Index:            1,
			Candidate:        &address.AddrV1{},
			Owner:            &address.AddrV1{},
			StakedAmount:     &big.Int{},
			StakedDuration:   time.Duration(1),
			CreateTime:       time.Time{},
			StakeStartTime:   time.Time{},
			UnstakeStartTime: time.Time{},
		},
	}
	b2 = []*staking.VoteBucket{
		&staking.VoteBucket{
			Index:            2,
			Candidate:        &address.AddrV1{},
			Owner:            &address.AddrV1{},
			StakedAmount:     &big.Int{},
			StakedDuration:   time.Duration(2),
			CreateTime:       time.Time{},
			StakeStartTime:   time.Time{},
			UnstakeStartTime: time.Time{},
		},
		&staking.VoteBucket{
			Index:            3,
			Candidate:        &address.AddrV1{},
			Owner:            &address.AddrV1{},
			StakedAmount:     &big.Int{},
			StakedDuration:   time.Duration(3),
			CreateTime:       time.Time{},
			StakeStartTime:   time.Time{},
			UnstakeStartTime: time.Time{},
		},
		&staking.VoteBucket{
			Index:            4,
			Candidate:        &address.AddrV1{},
			Owner:            &address.AddrV1{},
			StakedAmount:     &big.Int{},
			StakedDuration:   time.Duration(4),
			CreateTime:       time.Time{},
			StakeStartTime:   time.Time{},
			UnstakeStartTime: time.Time{},
		},
	}
	b3 = []*staking.VoteBucket{
		&staking.VoteBucket{
			Index:            5,
			Candidate:        &address.AddrV1{},
			Owner:            &address.AddrV1{},
			StakedAmount:     &big.Int{},
			StakedDuration:   time.Duration(5),
			CreateTime:       time.Time{},
			StakeStartTime:   time.Time{},
			UnstakeStartTime: time.Time{},
		},
		&staking.VoteBucket{
			Index:            6,
			Candidate:        &address.AddrV1{},
			Owner:            &address.AddrV1{},
			StakedAmount:     &big.Int{},
			StakedDuration:   time.Duration(6),
			CreateTime:       time.Time{},
			StakeStartTime:   time.Time{},
			UnstakeStartTime: time.Time{},
		},
		&staking.VoteBucket{
			Index:            7,
			Candidate:        &address.AddrV1{},
			Owner:            &address.AddrV1{},
			StakedAmount:     &big.Int{},
			StakedDuration:   time.Duration(7),
			CreateTime:       time.Time{},
			StakeStartTime:   time.Time{},
			UnstakeStartTime: time.Time{},
		},
		&staking.VoteBucket{
			Index:            8,
			Candidate:        &address.AddrV1{},
			Owner:            &address.AddrV1{},
			StakedAmount:     &big.Int{},
			StakedDuration:   time.Duration(8),
			CreateTime:       time.Time{},
			StakeStartTime:   time.Time{},
			UnstakeStartTime: time.Time{},
		},
	}
)

type srKV struct {
	kv db.KVStore
}

func newSRFromKVStore(kv db.KVStore) protocol.StateReader {
	return &srKV{kv}
}

func (sr *srKV) Height() (uint64, error) {
	return 0, nil
}

func (sr *srKV) State(s any, opts ...protocol.StateOption) (uint64, error) {
	cfg, err := protocol.CreateStateConfig(opts...)
	if err != nil {
		return 0, err
	}
	if len(cfg.Namespace) == 0 {
		cfg.Namespace = AccountKVNamespace
	}
	if cfg.Keys != nil {
		return 0, errors.Wrap(errors.New("not supported"), "Read state with keys option has not been implemented yet")
	}

	data, err := sr.kv.Get(cfg.Namespace, cfg.Key)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return 0, errors.Wrapf(state.ErrStateNotExist, "state of %x doesn't exist", cfg.Key)
		}
		return 0, errors.Wrapf(err, "error when getting the state of %x", cfg.Key)
	}
	if err := state.Deserialize(s, data); err != nil {
		return 0, errors.Wrapf(err, "error when deserializing state data into %T", s)
	}
	return 0, nil
}

func (sr *srKV) States(opts ...protocol.StateOption) (uint64, state.Iterator, error) {
	cfg, err := protocol.CreateStateConfig(opts...)
	if err != nil {
		return 0, nil, err
	}
	if len(cfg.Namespace) == 0 {
		cfg.Namespace = AccountKVNamespace
	}
	if cfg.Keys != nil {
		return 0, nil, errors.Wrap(errors.New("not supported"), "Read states with key option has not been implemented yet")
	}
	keys, values, err := db.ReadStates(sr.kv, cfg.Namespace, cfg.Keys)
	if err != nil {
		return 0, nil, err
	}
	iter, err := state.NewIterator(keys, values)
	if err != nil {
		return 0, nil, err
	}
	return 0, iter, nil
}

func (sr *srKV) ReadView(string) (protocol.View, error) {
	return &staking.ViewData{}, nil
}
