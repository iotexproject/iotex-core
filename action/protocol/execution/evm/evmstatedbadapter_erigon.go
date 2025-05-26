package evm

import (
	"context"
	"time"

	erigonchain "github.com/erigontech/erigon-lib/chain"
	libcommon "github.com/erigontech/erigon-lib/common"
	types2 "github.com/erigontech/erigon-lib/types"
	erigonstate "github.com/erigontech/erigon/core/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

// ErigonStateDBAdapter is an adapter for persisted both in erigon intra block state and statedb
// reads are based on statedb
type ErigonStateDBAdapter struct {
	*StateDBAdapter
	intra *erigonstate.IntraBlockState
}

// ErigonStateDBAdapterDryrun is an adapter for simulation
// reads are based on erigon intra block state, writes not persisted
type ErigonStateDBAdapterDryrun struct {
	*ErigonStateDBAdapter
}

// NewErigonStateDBAdapter creates a new ErigonStateDBAdapter
func NewErigonStateDBAdapter(adapter *StateDBAdapter,
	intra *erigonstate.IntraBlockState,
) *ErigonStateDBAdapter {
	adapter.newContract = func(addr hash.Hash160, account *state.Account) (Contract, error) {
		return newContractAdapter(addr, account, adapter.sm, intra, adapter.asyncContractTrie)
	}
	return &ErigonStateDBAdapter{
		StateDBAdapter: adapter,
		intra:          intra,
	}
}

// NewErigonStateDBAdapterDryrun creates a new ErigonStateDBAdapterDryrun
func NewErigonStateDBAdapterDryrun(adapter *StateDBAdapter,
	intra *erigonstate.IntraBlockState,
) *ErigonStateDBAdapterDryrun {
	a := NewErigonStateDBAdapter(adapter, intra)
	adapter.newContract = func(addr hash.Hash160, account *state.Account) (Contract, error) {
		return newContractErigon(addr, account, intra)
	}
	return &ErigonStateDBAdapterDryrun{
		ErigonStateDBAdapter: a,
	}
}

// SubBalance subtracts the balance of the given address by the given value
func (s *ErigonStateDBAdapter) SubBalance(evmAddr common.Address, v *uint256.Int) {
	s.StateDBAdapter.SubBalance(evmAddr, v)
	// balance updates for erigon will be done in statedb
}

// AddBalance adds the balance of the given address by the given value
func (s *ErigonStateDBAdapter) AddBalance(evmAddr common.Address, v *uint256.Int) {
	s.StateDBAdapter.AddBalance(evmAddr, v)
	// balance updates for erigon will be done in statedb
}

// SetCode set the code of the given address
func (s *ErigonStateDBAdapter) SetCode(evmAddr common.Address, c []byte) {
	s.StateDBAdapter.SetCode(evmAddr, c)
	// code updates for erigon will be done by Contract
}

// SetState set the state of the given address
func (s *ErigonStateDBAdapter) SetState(evmAddr common.Address, k common.Hash, v common.Hash) {
	s.StateDBAdapter.SetState(evmAddr, k, v)
	// state updates for erigon will be done by Contract
}

// CommitContracts commits the contracts to the state database
func (s *ErigonStateDBAdapter) CommitContracts() error {
	return s.StateDBAdapter.CommitContracts()
}

// RevertToSnapshot reverts the state database to the given snapshot
func (s *ErigonStateDBAdapter) RevertToSnapshot(sn int) {
	s.StateDBAdapter.RevertToSnapshot(sn)
	// revert for erigon will be done in statedb
}

// Snapshot creates a snapshot of the state database
func (s *ErigonStateDBAdapter) Snapshot() int {
	return s.StateDBAdapter.Snapshot()
	// snapshot for erigon will be done in statedb
}

// AddLog adds a log to the state database
func (s *ErigonStateDBAdapter) AddLog(l *types.Log) {
	s.StateDBAdapter.AddLog(l)
}

// AddPreimage adds a preimage to the state database
func (s *ErigonStateDBAdapter) AddPreimage(k common.Hash, v []byte) {
	s.StateDBAdapter.AddPreimage(k, v)
}

// CreateAccount creates a new account in the state database
func (s *ErigonStateDBAdapter) CreateAccount(evmAddr common.Address) {
	s.StateDBAdapter.CreateAccount(evmAddr)
	s.intra.CreateAccount(libcommon.Address(evmAddr), true)
}

// SetNonce sets the nonce of the given address
func (s *ErigonStateDBAdapter) SetNonce(evmAddr common.Address, n uint64) {
	s.StateDBAdapter.SetNonce(evmAddr, n)
	// nonce updates for erigon will be done in statedb
}

// AddRefund adds the refund of the given address
func (s *ErigonStateDBAdapter) AddRefund(r uint64) {
	s.StateDBAdapter.AddRefund(r)
	s.intra.AddRefund(r)
}

// SubRefund subtracts the refund of the given address
func (s *ErigonStateDBAdapter) SubRefund(r uint64) {
	s.StateDBAdapter.SubRefund(r)
	s.intra.SubRefund(r)
}

// SelfDestruct marks the given address for self-destruction
func (s *ErigonStateDBAdapter) SelfDestruct(evmAddr common.Address) {
	s.StateDBAdapter.SelfDestruct(evmAddr)
	s.intra.Selfdestruct(libcommon.Address(evmAddr))
}

// SelfDestruct6780 marks the given address for self-destruction
func (s *ErigonStateDBAdapter) Selfdestruct6780(evmAddr common.Address) {
	s.StateDBAdapter.Selfdestruct6780(evmAddr)
	s.intra.Selfdestruct6780(libcommon.Address(evmAddr))
}

// AddAddressToAccessList adds the given address to the access list
func (s *ErigonStateDBAdapter) AddAddressToAccessList(addr common.Address) {
	s.StateDBAdapter.AddAddressToAccessList(addr)
	s.intra.AddAddressToAccessList(libcommon.Address(addr))
}

// AddSlotToAccessList adds the given slot to the access list
func (s *ErigonStateDBAdapter) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	s.StateDBAdapter.AddSlotToAccessList(addr, slot)
	s.intra.AddSlotToAccessList(libcommon.Address(addr), libcommon.Hash(slot))
}

// Prepare prepares the state database
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
	s.intra.Prepare(NewErigonRules(&rules), libcommon.Address(sender), libcommon.Address(coinbase), d, prec, access, nil)
}

// GetCode gets the code saved in hash
func (stateDB *ErigonStateDBAdapterDryrun) GetCode(evmAddr common.Address) []byte {
	return stateDB.intra.GetCode(libcommon.Address(evmAddr))
}

// GetCodeSize gets the code size saved in hash
func (stateDB *ErigonStateDBAdapterDryrun) GetCodeSize(evmAddr common.Address) int {
	code := stateDB.intra.GetCodeSize(libcommon.Address(evmAddr))
	log.T(stateDB.ctx).Debug("Called GetCodeSize.", log.Hex("addrHash", evmAddr[:]))
	return code
}

// GetCodeHash gets the code hash saved in hash
func (stateDB *ErigonStateDBAdapterDryrun) GetCodeHash(evmAddr common.Address) common.Hash {
	codeHash := stateDB.intra.GetCodeHash(libcommon.Address(evmAddr))
	return common.Hash(codeHash)
}

// NewErigonRules creates a new Erigon rules
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
		IsPrague:           rules.IsPrague,
	}
}

// NewChainConfig creates a new chain config
func NewChainConfig(ctx context.Context) (*params.ChainConfig, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	g := genesis.MustExtractGenesisContext(ctx)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	return getChainConfig(g.Blockchain, blkCtx.BlockHeight, bcCtx.EvmNetworkID, func(height uint64) (*time.Time, error) {
		return blockHeightToTime(ctx, height)
	})
}
