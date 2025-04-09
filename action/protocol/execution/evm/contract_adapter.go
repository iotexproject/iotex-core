package evm

import (
	erigonstate "github.com/erigontech/erigon/core/state"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/state"
)

type contractReader interface {
	GetCommittedState(hash.Hash256) ([]byte, error)
	GetState(hash.Hash256) ([]byte, error)
	SetState(hash.Hash256, []byte) error
	GetCode() ([]byte, error)
	SelfState() *state.Account
}

type contractAdapter struct {
	contractReader
	trie   Contract
	erigon Contract
}

func newContractAdapter(addr hash.Hash160, account *state.Account, sm protocol.StateManager, intra *erigonstate.IntraBlockState, enableAsync bool) (Contract, error) {
	v1, err := newContract(addr, account, sm, enableAsync)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create contract")
	}
	v2, err := newContractErigon(addr, account, intra)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create contractV2")
	}
	c := &contractAdapter{
		contractReader: v1,
		trie:           v1,
		erigon:         v2,
	}
	return c, nil
}

func (c *contractAdapter) SetState(key hash.Hash256, value []byte) error {
	if err := c.trie.SetState(key, value); err != nil {
		return err
	}
	return c.erigon.SetState(key, value)
}

func (c *contractAdapter) SetCode(hash hash.Hash256, code []byte) {
	c.trie.SetCode(hash, code)
	c.erigon.SetCode(hash, code)
}

func (c *contractAdapter) Commit() error {
	if err := c.trie.Commit(); err != nil {
		return err
	}
	return c.erigon.Commit()
}

func (c *contractAdapter) LoadRoot() error {
	if err := c.trie.LoadRoot(); err != nil {
		return err
	}
	return c.erigon.LoadRoot()
}

// Iterator is only for debug
func (c *contractAdapter) Iterator() (trie.Iterator, error) {
	return nil, errors.New("not supported")
}

func (c *contractAdapter) Snapshot() Contract {
	v1 := c.trie.Snapshot()
	v2 := c.erigon.Snapshot()
	return &contractAdapter{
		contractReader: v1,
		trie:           v1,
		erigon:         v2,
	}
}
