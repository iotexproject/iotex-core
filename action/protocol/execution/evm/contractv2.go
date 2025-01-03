package evm

import (
	"math/big"

	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/hash"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	erigonstate "github.com/ledgerwatch/erigon/core/state"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/state"
)

type contractV2 struct {
	*state.Account
	dirtyCode  bool                       // contract's code has been set
	dirtyState bool                       // contract's account state has changed
	code       protocol.SerializableBytes // contract byte-code
	committed  map[hash.Hash256][]byte
	sm         protocol.StateReader
	intra      *erigonstate.IntraBlockState
	addr       hash.Hash160
}

func newContractV2(addr hash.Hash160, account *state.Account, sm protocol.StateReader, intra *erigonstate.IntraBlockState) (Contract, error) {
	c := &contractV2{
		Account:   account,
		committed: make(map[hash.Hash256][]byte),
		sm:        sm,
		intra:     intra,
		addr:      addr,
	}
	return c, nil
}

func (c *contractV2) GetCommittedState(key hash.Hash256) ([]byte, error) {
	if v, ok := c.committed[key]; ok {
		return v, nil
	}
	return c.GetState(key)

}
func (c *contractV2) GetState(key hash.Hash256) ([]byte, error) {
	k := libcommon.Hash(key)
	v := uint256.NewInt(0)
	c.intra.GetState(libcommon.Address(c.addr), &k, v)
	// Fix(erigon): return err if not exist
	if _, ok := c.committed[key]; !ok {
		c.committed[key] = v.Bytes()
	}
	return v.Bytes(), nil
}

func (c *contractV2) SetState(key hash.Hash256, value []byte) error {
	if _, ok := c.committed[key]; !ok {
		_, _ = c.GetState(key)
	}
	c.dirtyState = true
	k := libcommon.Hash(key)
	c.intra.SetState(libcommon.Address(c.addr), &k, *uint256.MustFromBig(big.NewInt(0).SetBytes(value)))
	return nil
}
func (c *contractV2) GetCode() ([]byte, error) {
	if c.code != nil {
		return c.code[:], nil
	}
	_, err := c.sm.State(&c.code, protocol.NamespaceOption(CodeKVNameSpace), protocol.KeyOption(c.Account.CodeHash))
	if err != nil {
		return nil, err
	}
	return c.code[:], nil
}
func (c *contractV2) SetCode(hash hash.Hash256, code []byte) {
	c.Account.CodeHash = hash[:]
	c.code = code
	c.dirtyCode = true
}

func (c *contractV2) SelfState() *state.Account {
	return c.Account
}

func (c *contractV2) Commit() error {
	if c.dirtyState {
		c.dirtyState = false
		// purge the committed value cache
		c.committed = nil
		c.committed = make(map[hash.Hash256][]byte)
	}
	if c.dirtyCode {
		c.dirtyCode = false
	}
	return nil
}
func (c *contractV2) LoadRoot() error {
	return nil
}

// Iterator is only for debug
func (c *contractV2) Iterator() (trie.Iterator, error) {
	return nil, errors.New("not supported")
}

func (c *contractV2) Snapshot() Contract {
	return &contractV2{
		Account:    c.Account.Clone(),
		dirtyCode:  c.dirtyCode,
		dirtyState: c.dirtyState,
		code:       c.code,
		committed:  c.committed,
		sm:         c.sm,
		intra:      c.intra,
	}
}
