package evm

import (
	erigonstate "github.com/erigontech/erigon/core/state"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

type (
	baseStateAccessor interface {
		Height() (uint64, error)
		BaseState(interface{}, ...protocol.StateOption) (uint64, error)
		BasePutState(interface{}, ...protocol.StateOption) (uint64, error)
		BaseDelState(...protocol.StateOption) (uint64, error)
		ReadView(string) (protocol.View, error)
	}

	proofStateManager struct {
		accessor baseStateAccessor
	}

	contractDryrunHybrid struct {
		proof      Contract
		erigon     Contract
		witnessErr error
	}
)

func newProofStateManager(sm protocol.StateManager) (protocol.StateManager, error) {
	accessor, ok := sm.(baseStateAccessor)
	if !ok {
		return nil, errors.New("state manager does not expose base-state reads for dry-run witness assembly")
	}
	return &proofStateManager{accessor: accessor}, nil
}

func (m *proofStateManager) Height() (uint64, error) {
	return m.accessor.Height()
}

func (m *proofStateManager) State(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	return m.accessor.BaseState(s, opts...)
}

func (m *proofStateManager) States(...protocol.StateOption) (uint64, state.Iterator, error) {
	return 0, nil, errors.Wrap(protocol.ErrUnimplemented, "proofStateManager.States")
}

func (m *proofStateManager) ReadView(name string) (protocol.View, error) {
	return m.accessor.ReadView(name)
}

func (m *proofStateManager) Snapshot() int {
	return 0
}

func (m *proofStateManager) Revert(int) error {
	return nil
}

func (m *proofStateManager) PutState(s interface{}, opts ...protocol.StateOption) (uint64, error) {
	return m.accessor.BasePutState(s, opts...)
}

func (m *proofStateManager) DelState(opts ...protocol.StateOption) (uint64, error) {
	return m.accessor.BaseDelState(opts...)
}

func (m *proofStateManager) WriteView(string, protocol.View) error {
	return errors.Wrap(protocol.ErrUnimplemented, "proofStateManager.WriteView")
}

func newContractDryrunHybrid(addr hash.Hash160, account *state.Account, sm protocol.StateManager, intra *erigonstate.IntraBlockState, enableAsync bool) (Contract, error) {
	proofSM, err := newProofStateManager(sm)
	if err != nil {
		return nil, err
	}
	proofAccount, err := accountutil.LoadAccountByHash160(proofSM, addr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load base account for dry-run witness assembly")
	}
	proofContract, err := newContract(addr, proofAccount, proofSM, enableAsync)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create proof contract for dry-run witness assembly")
	}
	erigonContract, err := newContractErigon(addr, account.Clone(), intra, sm)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create erigon contract for dry-run execution")
	}
	return &contractDryrunHybrid{
		proof:  proofContract,
		erigon: erigonContract,
	}, nil
}

func (c *contractDryrunHybrid) recordWitnessErr(err error) {
	if err == nil || c.witnessErr != nil {
		return
	}
	c.witnessErr = err
	log.L().Error("Recorded dry-run hybrid witness error.", zap.Error(err))
}

func (c *contractDryrunHybrid) GetCommittedState(key hash.Hash256) ([]byte, error) {
	_, proofErr := c.proof.GetCommittedState(key)
	if proofErr != nil && errors.Cause(proofErr) != trie.ErrNotExist {
		c.recordWitnessErr(proofErr)
	}
	return c.erigon.GetCommittedState(key)
}

func (c *contractDryrunHybrid) GetState(key hash.Hash256) ([]byte, error) {
	_, proofErr := c.proof.GetState(key)
	if proofErr != nil && errors.Cause(proofErr) != trie.ErrNotExist {
		c.recordWitnessErr(proofErr)
	}
	return c.erigon.GetState(key)
}

func (c *contractDryrunHybrid) SetState(key hash.Hash256, value []byte) error {
	if err := c.proof.SetState(key, value); err != nil {
		c.recordWitnessErr(err)
	}
	return c.erigon.SetState(key, value)
}

func (c *contractDryrunHybrid) BuildStorageWitness(access ContractStorageAccess) (*ContractStorageWitness, error) {
	if c.witnessErr != nil {
		log.L().Error("Dry-run hybrid witness assembly aborted due to prior observer error.", zap.Error(c.witnessErr))
		return nil, c.witnessErr
	}
	return c.proof.BuildStorageWitness(access)
}

func (c *contractDryrunHybrid) GetCode() ([]byte, error) {
	return c.erigon.GetCode()
}

func (c *contractDryrunHybrid) SetCode(codeHash hash.Hash256, code []byte) {
	c.proof.SetCode(codeHash, code)
	c.erigon.SetCode(codeHash, code)
}

func (c *contractDryrunHybrid) SelfState() *state.Account {
	return c.erigon.SelfState()
}

func (c *contractDryrunHybrid) Commit() error {
	return c.erigon.Commit()
}

func (c *contractDryrunHybrid) LoadRoot() error {
	if err := c.proof.LoadRoot(); err != nil {
		c.recordWitnessErr(err)
	}
	return c.erigon.LoadRoot()
}

func (c *contractDryrunHybrid) Iterator() (trie.Iterator, error) {
	return c.proof.Iterator()
}

func (c *contractDryrunHybrid) Snapshot() Contract {
	snapshot := &contractDryrunHybrid{
		proof:  c.proof.Snapshot(),
		erigon: c.erigon.Snapshot(),
	}
	snapshot.recordWitnessErr(c.witnessErr)
	return snapshot
}
