package evm

import (
	erigonstate "github.com/erigontech/erigon/core/state"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/state"
)

type contractAdapter struct {
	Contract
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
		Contract: v1,
		erigon:   v2,
	}
	return c, nil
}

func (c *contractAdapter) SetState(key hash.Hash256, value []byte) error {
	if err := c.Contract.SetState(key, value); err != nil {
		return err
	}
	return c.erigon.SetState(key, value)
}

func (c *contractAdapter) SetCode(hash hash.Hash256, code []byte) {
	c.Contract.SetCode(hash, code)
	c.erigon.SetCode(hash, code)
}

func (c *contractAdapter) Commit() error {
	if err := c.Contract.Commit(); err != nil {
		return err
	}
	return c.erigon.Commit()
}

func (c *contractAdapter) LoadRoot() error {
	if err := c.Contract.LoadRoot(); err != nil {
		return err
	}
	return c.erigon.LoadRoot()
}

// Iterator is only for debug
func (c *contractAdapter) Iterator() (trie.Iterator, error) {
	return nil, errors.New("not supported")
}

func (c *contractAdapter) Snapshot() Contract {
	v1 := c.Contract.Snapshot()
	v2 := c.erigon.Snapshot()
	return &contractAdapter{
		Contract: v1,
		erigon:   v2,
	}
}
