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
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

type contractV2 struct {
	*state.Account
	sm    protocol.StateReader
	intra *erigonstate.IntraBlockState
	addr  hash.Hash160
}

func newContractV2(addr hash.Hash160, account *state.Account, sm protocol.StateReader, intra *erigonstate.IntraBlockState) (Contract, error) {
	c := &contractV2{
		Account: account,
		sm:      sm,
		intra:   intra,
		addr:    addr,
	}
	return c, nil
}

func (c *contractV2) GetCommittedState(key hash.Hash256) ([]byte, error) {
	k := libcommon.Hash(key)
	v := uint256.NewInt(0)
	// Fix(erigon): return err if not exist
	c.intra.GetCommittedState(libcommon.Address(c.addr), &k, v)
	log.L().Debug("contractv2 GetCommittedState", log.Hex("key", key[:]), log.Hex("value", v.Bytes()))
	return v.Bytes(), nil
}

func (c *contractV2) GetState(key hash.Hash256) ([]byte, error) {
	k := libcommon.Hash(key)
	v := uint256.NewInt(0)
	// Fix(erigon): return err if not exist
	c.intra.GetState(libcommon.Address(c.addr), &k, v)
	log.L().Debug("contractv2 GetState", log.Hex("key", key[:]), log.Hex("value", v.Bytes()))
	return v.Bytes(), nil
}

func (c *contractV2) SetState(key hash.Hash256, value []byte) error {
	log.L().Debug("contractv2 SetState", log.Hex("key", key[:]), log.Hex("value", value))
	k := libcommon.Hash(key)
	c.intra.SetState(libcommon.Address(c.addr), &k, *uint256.MustFromBig(big.NewInt(0).SetBytes(value)))
	return nil
}

func (c *contractV2) GetCode() ([]byte, error) {
	code := c.intra.GetCode(libcommon.Address(c.addr))
	log.L().Debug("contractv2 GetCode", log.Hex("code", code))
	return code, nil
}

func (c *contractV2) SetCode(hash hash.Hash256, code []byte) {
	c.intra.SetCode(libcommon.Address(c.addr), code)
	eh := c.intra.GetCodeHash(libcommon.Address(c.addr))
	log.L().Debug("contractv2 SetCode", log.Hex("erigonhash", eh[:]), log.Hex("iotexhash", hash[:]))
}

func (c *contractV2) SelfState() *state.Account {
	acc := &state.Account{}
	acc.SetPendingNonce(c.intra.GetNonce(libcommon.Address(c.addr)))
	acc.AddBalance(c.intra.GetBalance(libcommon.Address(c.addr)).ToBig())
	codeHash := c.intra.GetCodeHash(libcommon.Address(c.addr))
	acc.CodeHash = codeHash[:]
	return acc
}

func (c *contractV2) Commit() error {
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
		Account: c.Account.Clone(),
		sm:      c.sm,
		intra:   c.intra,
		addr:    c.addr,
	}
}
