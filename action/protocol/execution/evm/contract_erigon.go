package evm

import (
	"math/big"

	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/hash"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	erigonstate "github.com/ledgerwatch/erigon/core/state"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

type contractErigon struct {
	*state.Account
	intra *erigonstate.IntraBlockState
	addr  hash.Hash160
}

func newContractErigon(addr hash.Hash160, account *state.Account, intra *erigonstate.IntraBlockState) (Contract, error) {
	c := &contractErigon{
		Account: account,
		intra:   intra,
		addr:    addr,
	}
	return c, nil
}

func (c *contractErigon) GetCommittedState(key hash.Hash256) ([]byte, error) {
	k := libcommon.Hash(key)
	v := uint256.NewInt(0)
	c.intra.GetCommittedState(libcommon.Address(c.addr), &k, v)
	return v.Bytes(), nil
}

func (c *contractErigon) GetState(key hash.Hash256) ([]byte, error) {
	k := libcommon.Hash(key)
	v := uint256.NewInt(0)
	c.intra.GetState(libcommon.Address(c.addr), &k, v)
	return v.Bytes(), nil
}

func (c *contractErigon) SetState(key hash.Hash256, value []byte) error {
	k := libcommon.Hash(key)
	c.intra.SetState(libcommon.Address(c.addr), &k, *uint256.MustFromBig(big.NewInt(0).SetBytes(value)))
	return nil
}

func (c *contractErigon) GetCode() ([]byte, error) {
	return c.intra.GetCode(libcommon.Address(c.addr)), nil
}

func (c *contractErigon) SetCode(hash hash.Hash256, code []byte) {
	c.intra.SetCode(libcommon.Address(c.addr), code)
	eh := c.intra.GetCodeHash(libcommon.Address(c.addr))
	log.L().Debug("SetCode", log.Hex("erigonhash", eh[:]), log.Hex("iotexhash", hash[:]))
}

func (c *contractErigon) SelfState() *state.Account {
	acc := &state.Account{}
	acc.SetPendingNonce(c.intra.GetNonce(libcommon.Address(c.addr)))
	acc.AddBalance(c.intra.GetBalance(libcommon.Address(c.addr)).ToBig())
	codeHash := c.intra.GetCodeHash(libcommon.Address(c.addr))
	acc.CodeHash = codeHash[:]
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
