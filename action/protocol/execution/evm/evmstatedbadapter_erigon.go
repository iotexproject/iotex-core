package evm

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	erigonchain "github.com/ledgerwatch/erigon-lib/chain"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	types2 "github.com/ledgerwatch/erigon-lib/types"
	erigonstate "github.com/ledgerwatch/erigon/core/state"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

type ErigonStateDBAdapter struct {
	*StateDBAdapter
	intra *erigonstate.IntraBlockState
}

type ErigonStateDBAdapterDryrun struct {
	*ErigonStateDBAdapter
}

func NewErigonStateDBAdapter(adapter *StateDBAdapter,
	intra *erigonstate.IntraBlockState,
	chainRules *erigonchain.Rules,
) *ErigonStateDBAdapter {
	adapter.newContract = func(addr hash.Hash160, account *state.Account) (Contract, error) {
		return newContractAdapter(addr, account, adapter.sm, intra, adapter.asyncContractTrie)
	}
	return &ErigonStateDBAdapter{
		StateDBAdapter: adapter,
		intra:          intra,
	}
}

func NewErigonStateDBAdapterDryrun(adapter *StateDBAdapter,
	intra *erigonstate.IntraBlockState,
	chainRules *erigonchain.Rules,
) *ErigonStateDBAdapterDryrun {
	a := NewErigonStateDBAdapter(adapter, intra, chainRules)
	adapter.newContract = func(addr hash.Hash160, account *state.Account) (Contract, error) {
		return newContractErigon(addr, account, intra)
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
}

func (s *ErigonStateDBAdapter) AddBalance(evmAddr common.Address, v *uint256.Int) {
	s.StateDBAdapter.AddBalance(evmAddr, v)
}

func (s *ErigonStateDBAdapter) SetNonce(evmAddr common.Address, n uint64) {
	s.StateDBAdapter.SetNonce(evmAddr, n)
	s.intra.SetNonce(libcommon.Address(evmAddr), n)
}

// SetCode the reason why not call intra.SetCode is because it will be called by contract
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

// SetState the reason why not call intra.SetState is because it will be called by contract
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
	return nil
}

// RevertToSnapshot the reason why not call intra.RevertToSnapshot is because it will be called by sm
func (s *ErigonStateDBAdapter) RevertToSnapshot(sn int) {
	s.StateDBAdapter.RevertToSnapshot(sn)
}

// Snapshot the reason why not call intra.Snapshot is because it will be called by sm
func (s *ErigonStateDBAdapter) Snapshot() int {
	return s.StateDBAdapter.Snapshot()
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

func NewErigonRules(rules *params.Rules) *erigonchain.Rules {
	return &erigonchain.Rules{
		ChainID:            rules.ChainID,
		IsHomestead:        rules.IsHomestead,
		IsTangerineWhistle: rules.IsByzantium,
		IsSpuriousDragon:   rules.IsByzantium,
		IsByzantium:        rules.IsByzantium,
		IsConstantinople:   rules.IsConstantinople,
		IsPetersburg:       rules.IsPetersburg,
		IsIstanbul:         rules.IsIstanbul,
		IsBerlin:           rules.IsBerlin,
		IsLondon:           rules.IsLondon,
		IsShanghai:         rules.IsShanghai,
		IsCancun:           rules.IsCancun,
		IsNapoli:           false,
		IsPrague:           false,
		IsOsaka:            false,
		IsAura:             false,
	}
}

func NewChainConfig(g genesis.Blockchain, height uint64, id uint32, getBlockTime GetBlockTime) (*params.ChainConfig, error) {
	return getChainConfig(g, height, id, getBlockTime)
}
