package evm

import (
	"fmt"
	"math/big"

	libcommon "github.com/erigontech/erigon-lib/common"
	erigonstate "github.com/erigontech/erigon/core/state"
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

type contractErigon struct {
	*state.Account
	intra *erigonstate.IntraBlockState
	sr    protocol.StateReader
	addr  hash.Hash160
}

func newContractErigon(addr hash.Hash160, account *state.Account, intra *erigonstate.IntraBlockState, sr protocol.StateReader) (Contract, error) {
	c := &contractErigon{
		Account: account,
		intra:   intra,
		addr:    addr,
		sr:      sr,
	}
	return c, nil
}

func (c *contractErigon) GetCommittedState(key hash.Hash256) ([]byte, error) {
	k := libcommon.Hash(key)
	v := uint256.NewInt(0)
	c.intra.GetCommittedState(libcommon.Address(c.addr), &k, v)
	if v.IsZero() {
		c.intra.GetState(libcommon.Address(c.addr), &k, v)
	}
	h := hash.BytesToHash256(v.Bytes())
	fmt.Printf("GetCommittedState(erigon): contract %x codehash %x key %x value %x\n", c.addr[:], c.CodeHash[:], key[:], h[:])
	return h[:], nil
}

func (c *contractErigon) GetState(key hash.Hash256) ([]byte, error) {
	k := libcommon.Hash(key)
	v := uint256.NewInt(0)
	c.intra.GetState(libcommon.Address(c.addr), &k, v)
	h := hash.BytesToHash256(v.Bytes())
	fmt.Printf("GetState(erigon): contract %x codehash %x key %x value %x\n", c.addr[:], c.CodeHash[:], key[:], h[:])
	return h[:], nil
}

func (c *contractErigon) SetState(key hash.Hash256, value []byte) error {
	k := libcommon.Hash(key)
	fmt.Printf("SetState(erigon): contract %x codehash %x key %x value %x\n", c.addr[:], c.CodeHash[:], key[:], value)
	c.intra.SetState(libcommon.Address(c.addr), &k, *uint256.MustFromBig(big.NewInt(0).SetBytes(value)))
	return nil
}

func (c *contractErigon) GetCode() ([]byte, error) {
	return c.intra.GetCode(libcommon.Address(c.addr)), nil
}

func (c *contractErigon) SetCode(hash hash.Hash256, code []byte) {
	c.intra.SetCode(libcommon.Address(c.addr), code)
}

func (c *contractErigon) SelfState() *state.Account {
	acc := &state.Account{}
	_, err := c.sr.State(acc, protocol.LegacyKeyOption(c.addr), protocol.ErigonStoreOnlyOption())
	if err != nil {
		log.S().Panicf("failed to load account %x: %v", c.addr, err)
	}
	return acc
}

func (c *contractErigon) Commit() error {
	return nil
}

func (c *contractErigon) LoadRoot() error {
	return nil
}

// Iterator is only for debug
func (c *contractErigon) Iterator() (trie.Iterator, error) {
	return nil, errors.New("not supported")
}

func (c *contractErigon) Snapshot() Contract {
	return &contractErigon{
		Account: c.Account.Clone(),
		intra:   c.intra,
		addr:    c.addr,
	}
}
