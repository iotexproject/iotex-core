package evm

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	erigonchain "github.com/ledgerwatch/erigon-lib/chain"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	types2 "github.com/ledgerwatch/erigon-lib/types"
	erigonstate "github.com/ledgerwatch/erigon/core/state"
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

type ErigonStateDBAdapterDryrun struct {
	*ErigonStateDBAdapter
}

func NewErigonStateDBAdapter(adapter *StateDBAdapter,
	rw erigonstate.StateWriter,
	intra *erigonstate.IntraBlockState,
	chainRules *erigonchain.Rules,
) *ErigonStateDBAdapter {
	adapter.newContract = func(addr hash.Hash160, account *state.Account) (Contract, error) {
		return newContractV3(addr, account, adapter.sm, intra, adapter.asyncContractTrie)
	}
	return &ErigonStateDBAdapter{
		StateDBAdapter: adapter,
		rw:             rw,
		intra:          intra,
		chainRules:     chainRules,
	}
}

func NewErigonStateDBAdapterDryrun(adapter *StateDBAdapter,
	rw erigonstate.StateWriter,
	intra *erigonstate.IntraBlockState,
	chainRules *erigonchain.Rules,
) *ErigonStateDBAdapterDryrun {
	a := NewErigonStateDBAdapter(adapter, rw, intra, chainRules)
	adapter.newContract = func(addr hash.Hash160, account *state.Account) (Contract, error) {
		return newContractV2(addr, account, adapter.sm, intra)
	}
	return &ErigonStateDBAdapterDryrun{
		ErigonStateDBAdapter: a,
	}
}

func (s *ErigonStateDBAdapter) CreateAccount(evmAddr common.Address) {
	s.StateDBAdapter.CreateAccount(evmAddr)
	s.intra.CreateAccount(libcommon.Address(evmAddr), true)
}

func (s *ErigonStateDBAdapter) SubBalance(evmAddr common.Address, v *uint256.Int) {
	s.StateDBAdapter.SubBalance(evmAddr, v)
	// s.intra.SubBalance(libcommon.Address(evmAddr), v)
}

func (s *ErigonStateDBAdapter) AddBalance(evmAddr common.Address, v *uint256.Int) {
	s.StateDBAdapter.AddBalance(evmAddr, v)
	// s.intra.AddBalance(libcommon.Address(evmAddr), v)
}

func (s *ErigonStateDBAdapter) SetNonce(evmAddr common.Address, n uint64) {
	s.StateDBAdapter.SetNonce(evmAddr, n)
	s.intra.SetNonce(libcommon.Address(evmAddr), n)
}

func (s *ErigonStateDBAdapter) SetCode(evmAddr common.Address, c []byte) {
	s.StateDBAdapter.SetCode(evmAddr, c)
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
	err := s.StateDBAdapter.CommitContracts()
	if err != nil {
		return err
	}
	// err = s.intra.FinalizeTx(s.chainRules, s.rw)
	// if err != nil {
	// 	return errors.Wrap(err, "failed to finalize tx")
	// }
	return nil
}

func (s *ErigonStateDBAdapter) RevertToSnapshot(sn int) {
	log.L().Debug("erigon adapter revert to snapshot", zap.Int("sn", sn), zap.Int("isn", sn+s.snDiff))
	s.StateDBAdapter.RevertToSnapshot(sn)
}

func (s *ErigonStateDBAdapter) Snapshot() int {
	sn := s.StateDBAdapter.Snapshot()
	return sn
}

func (s *ErigonStateDBAdapter) AddAddressToAccessList(addr common.Address) {
	s.StateDBAdapter.AddAddressToAccessList(addr)
	s.intra.AddAddressToAccessList(libcommon.Address(addr))
}

func (s *ErigonStateDBAdapter) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	s.StateDBAdapter.AddSlotToAccessList(addr, slot)
	s.intra.AddSlotToAccessList(libcommon.Address(addr), libcommon.Hash(slot))
}
func (s *ErigonStateDBAdapter) Prepare(rules params.Rules, sender, coinbase common.Address, dest *common.Address, precompiles []common.Address, txAccesses types.AccessList) {
	s.StateDBAdapter.Prepare(rules, sender, coinbase, dest, precompiles, txAccesses)
	var (
		d      *libcommon.Address
		prec   = make([]libcommon.Address, len(precompiles))
		access = types2.AccessList{}
	)
	if dest != nil {
		d = new(libcommon.Address)
		*d = libcommon.Address(*dest)
	}
	for i, p := range precompiles {
		prec[i] = libcommon.Address(p)
	}
	for _, a := range txAccesses {
		acl := types2.AccessTuple{
			Address: libcommon.Address(a.Address),
		}
		for _, s := range a.StorageKeys {
			acl.StorageKeys = append(acl.StorageKeys, libcommon.Hash(s))
		}
		access = append(access, acl)
	}
	s.intra.Prepare(NewErigonRules(&rules), libcommon.Address(sender), libcommon.Address(coinbase), d, prec, access)
}

func (s *ErigonStateDBAdapter) AddLog(l *types.Log) {
	s.StateDBAdapter.AddLog(l)
}

func (s *ErigonStateDBAdapter) AddPreimage(k common.Hash, v []byte) {
	s.StateDBAdapter.AddPreimage(k, v)
}

func (stateDB *ErigonStateDBAdapterDryrun) GetCode(evmAddr common.Address) []byte {
	return stateDB.intra.GetCode(libcommon.Address(evmAddr))
}

// GetCodeSize gets the code size saved in hash
func (stateDB *ErigonStateDBAdapterDryrun) GetCodeSize(evmAddr common.Address) int {
	code := stateDB.intra.GetCodeSize(libcommon.Address(evmAddr))
	log.T(stateDB.ctx).Debug("Called GetCodeSize.", log.Hex("addrHash", evmAddr[:]))
	return code
}

func (stateDB *ErigonStateDBAdapterDryrun) GetCodeHash(evmAddr common.Address) common.Hash {
	codeHash := stateDB.intra.GetCodeHash(libcommon.Address(evmAddr))
	log.T(stateDB.ctx).Debug("ErigonStateDBAdapterDryrun Called GetCodeHash.", log.Hex("addrHash", evmAddr[:]), log.Hex("codeHash", codeHash[:]))
	return common.Hash(codeHash)
}
