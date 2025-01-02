package evm

import (
	"github.com/ethereum/go-ethereum/core/vm"
	erigonchain "github.com/ledgerwatch/erigon-lib/chain"
	erigonstate "github.com/ledgerwatch/erigon/core/state"
	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/state"
)

type StateDB interface {
	vm.StateDB

	CommitContracts() error
	Logs() []*action.Log
	TransactionLogs() []*action.TransactionLog
	clear()
	Error() error
}

type ErigonStateDBAdapter struct {
	*StateDBAdapter
	rw         erigonstate.StateWriter
	intra      *erigonstate.IntraBlockState
	chainRules *erigonchain.Rules
}

func NewErigonStateDBAdapter(adapter *StateDBAdapter,
	rw *erigonstate.DbStateWriter,
	intra *erigonstate.IntraBlockState,
	chainRules *erigonchain.Rules,
) *ErigonStateDBAdapter {
	adapter.newContract = func(addr hash.Hash160, account *state.Account) (Contract, error) {
		return newContractV2(addr, account, adapter.sm, intra)
	}
	return &ErigonStateDBAdapter{
		StateDBAdapter: adapter,
		rw:             rw,
		intra:          intra,
		chainRules:     chainRules,
	}
}

func (s *ErigonStateDBAdapter) CommitContracts() error {
	err := s.intra.FinalizeTx(s.chainRules, s.rw)
	if err != nil {
		return errors.Wrap(err, "failed to finalize tx")
	}
	return nil
}
