package evm

import (
	"github.com/iotexproject/go-pkgs/hash"
	erigonstate "github.com/ledgerwatch/erigon/core/state"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

type contractReader interface {
	GetCommittedState(hash.Hash256) ([]byte, error)
	GetState(hash.Hash256) ([]byte, error)
	SetState(hash.Hash256, []byte) error
	GetCode() ([]byte, error)
	SelfState() *state.Account
}

type contractV3 struct {
	contractReader
	v1 *contract
	v2 *contractV2
}

func newContractV3(addr hash.Hash160, account *state.Account, sm protocol.StateManager, intra *erigonstate.IntraBlockState, enableAsync bool) (Contract, error) {
	v1, err := newContract(addr, account, sm, enableAsync)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create contract")
	}
	v2, err := newContractV2(addr, account, sm, intra)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create contractV2")
	}
	c := &contractV3{
		contractReader: v1,
		v1:             v1.(*contract),
		v2:             v2.(*contractV2),
	}
	return c, nil
}

func (c *contractV3) SetState(key hash.Hash256, value []byte) error {
	log.L().Debug("contractv3 set state", log.Hex("key", key[:]), log.Hex("value", value))
	if err := c.v1.SetState(key, value); err != nil {
		return err
	}
	return c.v2.SetState(key, value)
}

func (c *contractV3) SetCode(hash hash.Hash256, code []byte) {
	log.L().Debug("contractv3 set code", log.Hex("hash", hash[:]), log.Hex("code", code))
	c.v1.SetCode(hash, code)
	c.v2.SetCode(hash, code)
}

func (c *contractV3) Commit() error {
	log.L().Debug("contractv3 commit")
	if err := c.v1.Commit(); err != nil {
		return err
	}
	return c.v2.Commit()
}

func (c *contractV3) LoadRoot() error {
	log.L().Debug("contractv3 load root")
	if err := c.v1.LoadRoot(); err != nil {
		return err
	}
	return c.v2.LoadRoot()
}

// Iterator is only for debug
func (c *contractV3) Iterator() (trie.Iterator, error) {
	return nil, errors.New("not supported")
}

func (c *contractV3) Snapshot() Contract {
	log.L().Debug("contractv3 snapshot")
	v1 := c.v1.Snapshot().(*contract)
	v2 := c.v2.Snapshot().(*contractV2)
	return &contractV3{
		contractReader: v1,
		v1:             v1,
		v2:             v2,
	}
}
