package evm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	erigonchain "github.com/ledgerwatch/erigon-lib/chain"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	erigonstate "github.com/ledgerwatch/erigon/core/state"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
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
	snDiff     int
}

func NewErigonStateDBAdapter(adapter *StateDBAdapter,
	rw erigonstate.StateWriter,
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

func (s *ErigonStateDBAdapter) CreateAccount(evmAddr common.Address) {
	s.StateDBAdapter.CreateAccount(evmAddr)
	s.intra.CreateAccount(libcommon.Address(evmAddr), true)
}

func (s *ErigonStateDBAdapter) SubBalance(evmAddr common.Address, v *uint256.Int) {
	s.StateDBAdapter.SubBalance(evmAddr, v)
	s.intra.SubBalance(libcommon.Address(evmAddr), v)
}

func (s *ErigonStateDBAdapter) AddBalance(evmAddr common.Address, v *uint256.Int) {
	s.StateDBAdapter.AddBalance(evmAddr, v)
	s.intra.AddBalance(libcommon.Address(evmAddr), v)
}

func (s *ErigonStateDBAdapter) SetNonce(evmAddr common.Address, n uint64) {
	s.StateDBAdapter.SetNonce(evmAddr, n)
	s.intra.SetNonce(libcommon.Address(evmAddr), n)
}

func (s *ErigonStateDBAdapter) SetCode(evmAddr common.Address, c []byte) {
	s.StateDBAdapter.SetCode(evmAddr, c)
	s.intra.SetCode(libcommon.Address(evmAddr), c)
}

func (s *ErigonStateDBAdapter) AddRefund(r uint64) {
	s.StateDBAdapter.AddRefund(r)
	s.intra.AddRefund(r)
}
func (s *ErigonStateDBAdapter) SubRefund(r uint64) {
	s.StateDBAdapter.SubRefund(r)
	s.intra.SubRefund(r)
}
func (s *ErigonStateDBAdapter) SetState(evmAddr common.Address, k common.Hash, v common.Hash) {
	s.StateDBAdapter.SetState(evmAddr, k, v)
	key := libcommon.Hash(k)
	s.intra.SetState(libcommon.Address(evmAddr), &key, *uint256.MustFromBig(big.NewInt(0).SetBytes(v[:])))
}

func (s *ErigonStateDBAdapter) SelfDestruct(evmAddr common.Address) {
	s.StateDBAdapter.SelfDestruct(evmAddr)
	s.intra.Selfdestruct(libcommon.Address(evmAddr))
}

func (s *ErigonStateDBAdapter) Selfdestruct6780(evmAddr common.Address) {
	s.StateDBAdapter.Selfdestruct6780(evmAddr)
	s.intra.Selfdestruct6780(libcommon.Address(evmAddr))
}

func (s *ErigonStateDBAdapter) CommitContracts() error {
	log.L().Debug("intraBlockState Committing contracts", zap.Uint64("height", s.StateDBAdapter.blockHeight))
	err := s.intra.FinalizeTx(s.chainRules, s.rw)
	if err != nil {
		return errors.Wrap(err, "failed to finalize tx")
	}
	return nil
}

func (s *ErigonStateDBAdapter) RevertToSnapshot(sn int) {
	s.StateDBAdapter.RevertToSnapshot(sn)
	s.intra.RevertToSnapshot(sn + s.snDiff)
}

func (s *ErigonStateDBAdapter) Snapshot() int {
	sn := s.StateDBAdapter.Snapshot()
	isn := s.intra.Snapshot()
	diff := isn - sn
	if s.snDiff != 0 && diff != s.snDiff {
		log.L().Panic("snapshot diff changed", zap.Int("old", s.snDiff), zap.Int("new", diff))
	}
	s.snDiff = diff
	return sn
}
