package erigonstore

import (
	"context"
	"encoding/hex"
	"math"
	"math/big"
	"time"

	"github.com/erigontech/erigon-lib/chain"
	erigonComm "github.com/erigontech/erigon-lib/common"
	erigonstate "github.com/erigontech/erigon/core/state"
	"github.com/erigontech/erigon/core/types/accounts"
	erigonAcc "github.com/erigontech/erigon/core/types/accounts"
	"github.com/erigontech/erigon/core/vm"
	"github.com/erigontech/erigon/core/vm/evmtypes"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account/accountpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	iotexevm "github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

type (
	contractBackend struct {
		intraBlockState *erigonstate.IntraBlockState
		org             erigonstate.StateReader

		// helper fields
		height       uint64
		timestamp    time.Time
		producer     erigonComm.Address
		g            *genesis.Genesis
		evmNetworkID uint32
	}
)

// NewContractBackend creates a new contract backend for system contract interaction
func NewContractBackend(intraBlockState *erigonstate.IntraBlockState, org erigonstate.StateReader, height uint64, timestamp time.Time, producer address.Address, g *genesis.Genesis, evmNetworkID uint32) *contractBackend {
	return &contractBackend{
		intraBlockState: intraBlockState,
		org:             org,
		height:          height,
		timestamp:       timestamp,
		g:               g,
		evmNetworkID:    evmNetworkID,
		producer:        erigonComm.BytesToAddress(producer.Bytes()),
	}
}

func (backend *contractBackend) Call(callMsg *ethereum.CallMsg) ([]byte, error) {
	return backend.call(callMsg, erigonstate.New(&intraStateReader{backend.intraBlockState, backend.org}))
}

func (backend *contractBackend) Handle(callMsg *ethereum.CallMsg) error {
	_, err := backend.call(callMsg, backend.intraBlockState)
	return err
}

func (backend *contractBackend) Deploy(callMsg *ethereum.CallMsg) (address.Address, error) {
	evm, err := backend.prepare(backend.intraBlockState)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare EVM for contract deployment")
	}
	ret, addr, leftGas, err := evm.Create(vm.AccountRef(callMsg.From), callMsg.Data, callMsg.Gas, uint256.MustFromBig(callMsg.Value), true)
	if err != nil {
		if errors.Is(err, vm.ErrExecutionReverted) {
			revertMsg := iotexevm.ExtractRevertMessage(ret)
			log.L().Error("EVM deployment reverted",
				zap.String("from", callMsg.From.String()),
				zap.String("data", hex.EncodeToString(callMsg.Data)),
				zap.String("revertMessage", revertMsg),
				zap.String("returnData", hex.EncodeToString(ret)),
			)
			return nil, errors.Wrapf(err, "deployment reverted: %s", revertMsg)
		}
		return nil, errors.Wrap(err, "failed to deploy contract")
	}
	log.L().Debug("EVM deployment result",
		zap.String("from", callMsg.From.String()),
		zap.String("data", hex.EncodeToString(callMsg.Data)),
		zap.String("ret", hex.EncodeToString(ret)),
		zap.String("address", addr.String()),
		zap.Uint64("gasLeft", leftGas),
	)

	return address.FromBytes(addr.Bytes())
}

func (backend *contractBackend) Exists(addr address.Address) bool {
	return backend.intraBlockState.Exist(erigonComm.BytesToAddress(addr.Bytes()))
}

func (backend *contractBackend) PutAccount(_addr address.Address, acc *state.Account) error {
	addr := erigonComm.BytesToAddress(_addr.Bytes())
	if !backend.intraBlockState.Exist(addr) {
		backend.intraBlockState.CreateAccount(addr, acc.IsContract())
	}
	backend.intraBlockState.SetBalance(addr, uint256.MustFromBig(acc.Balance))
	ctx := protocol.WithFeatureCtx(protocol.WithBlockCtx(genesis.WithGenesisContext(context.Background(), *backend.g), protocol.BlockCtx{BlockHeight: backend.height}))
	fCtx := protocol.MustGetFeatureCtx(ctx)
	nonce := acc.PendingNonce()
	if fCtx.UseZeroNonceForFreshAccount {
		nonce = acc.PendingNonceConsideringFreshAccount()
	}
	backend.intraBlockState.SetNonce(addr, nonce)
	// store other fields in the account storage contract
	pbAcc := acc.ToProto()
	pbAcc.Balance = ""
	pbAcc.Nonce = 0
	data, err := proto.Marshal(pbAcc)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal account %x", addr.Bytes())
	}
	contract, err := systemcontracts.NewGenericStorageContract(backend.accountContractAddr(), backend)
	if err != nil {
		return errors.Wrapf(err, "failed to create account storage contract for address %x", addr.Bytes())
	}
	return contract.Put(_addr.Bytes(), systemcontracts.GenericValue{PrimaryData: data})
}

func (backend *contractBackend) Account(_addr address.Address) (*state.Account, error) {
	addr := erigonComm.BytesToAddress(_addr.Bytes())
	if !backend.intraBlockState.Exist(addr) {
		return nil, errors.Wrapf(state.ErrStateNotExist, "address: %x", addr.Bytes())
	}
	// load other fields from the account storage contract
	contract, err := systemcontracts.NewGenericStorageContract(backend.accountContractAddr(), backend)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create account storage contract for address %x", addr.Bytes())
	}
	value, err := contract.Get(_addr.Bytes())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get account data for address %x", addr.Bytes())
	}
	if !value.KeyExists {
		return nil, errors.Errorf("account info not found for address %x", addr.Bytes())
	}
	pbAcc := &accountpb.Account{}
	if err := proto.Unmarshal(value.Value.PrimaryData, pbAcc); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal account data for address %x", addr.Bytes())
	}

	balance := backend.intraBlockState.GetBalance(addr)
	nonce := backend.intraBlockState.GetNonce(addr)
	pbAcc.Balance = balance.String()
	switch pbAcc.Type {
	case accountpb.AccountType_ZERO_NONCE:
		pbAcc.Nonce = nonce
	case accountpb.AccountType_DEFAULT:
		pbAcc.Nonce = nonce - 1
	default:
		return nil, errors.Errorf("unknown account type %v for address %x", pbAcc.Type, addr.Bytes())
	}

	if ch := backend.intraBlockState.GetCodeHash(addr); !accounts.IsEmptyCodeHash(ch) {
		pbAcc.CodeHash = ch.Bytes()
	}
	acc := &state.Account{}
	acc.FromProto(pbAcc)
	return acc, nil
}

func (backend *contractBackend) accountContractAddr() common.Address {
	return common.BytesToAddress(systemcontracts.SystemContracts[systemcontracts.AccountInfoContractIndex].Address.Bytes())
}

func (backend *contractBackend) prepare(intra evmtypes.IntraBlockState) (*vm.EVM, error) {
	blkCtxE := evmtypes.BlockContext{
		CanTransfer: func(state evmtypes.IntraBlockState, addr erigonComm.Address, amount *uint256.Int) bool {
			log.L().Debug("CanTransfer called in erigon genesis state creation",
				zap.String("address", addr.String()),
				zap.String("amount", amount.String()),
			)
			return true
		},
		Transfer: func(state evmtypes.IntraBlockState, from erigonComm.Address, to erigonComm.Address, amount *uint256.Int, bailout bool) {
			log.L().Debug("Transfer called in erigon genesis state creation",
				zap.String("from", from.String()),
				zap.String("to", to.String()),
				zap.String("amount", amount.String()),
			)
			return
		},
		GetHash: func(block uint64) erigonComm.Hash {
			log.L().Debug("GetHash called in erigon genesis state creation",
				zap.Uint64("block", block),
			)
			return erigonComm.Hash{}
		},
		PostApplyMessage: func(ibs evmtypes.IntraBlockState, sender erigonComm.Address, coinbase erigonComm.Address, result *evmtypes.ExecutionResult) {
			log.L().Debug("PostApplyMessage called in erigon genesis state creation",
				zap.String("sender", sender.String()),
				zap.String("coinbase", coinbase.String()),
			)
			return
		},
		Coinbase:    backend.producer,
		GasLimit:    math.MaxUint64,
		MaxGasLimit: true,
		BlockNumber: backend.height,
		Time:        uint64(backend.timestamp.Unix()),
		Difficulty:  big.NewInt(50),
		BaseFee:     nil,
		PrevRanDao:  nil,
		BlobBaseFee: nil,
	}
	txCtxE := evmtypes.TxContext{
		TxHash:     erigonComm.Hash{},
		Origin:     erigonComm.Address{},
		GasPrice:   uint256.NewInt(0),
		BlobFee:    nil,
		BlobHashes: nil,
	}
	ctx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{
		BlockHeight:    backend.height,
		BlockTimeStamp: backend.timestamp})
	ctx = genesis.WithGenesisContext(ctx, *backend.g)
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
		GetBlockTime: func(u uint64) (time.Time, error) {
			interval := 2500 * time.Millisecond
			return backend.timestamp.Add(interval * time.Duration(u-backend.height)), nil
		},
		EvmNetworkID: backend.evmNetworkID,
	})
	chainCfg, err := evm.NewChainConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create chain config")
	}
	var (
		shanghaiTime *big.Int
		cancunTime   *big.Int
	)
	if chainCfg.ShanghaiTime != nil {
		shanghaiTime = big.NewInt(int64(*chainCfg.ShanghaiTime))
	}
	if chainCfg.CancunTime != nil {
		cancunTime = big.NewInt(int64(*chainCfg.CancunTime))
	}
	chainConfig := &chain.Config{
		HomesteadBlock:        chainCfg.ConstantinopleBlock,
		DAOForkBlock:          chainCfg.ConstantinopleBlock,
		TangerineWhistleBlock: chainCfg.ConstantinopleBlock,
		SpuriousDragonBlock:   chainCfg.ConstantinopleBlock,
		ByzantiumBlock:        chainCfg.ConstantinopleBlock,
		ConstantinopleBlock:   chainCfg.ConstantinopleBlock,
		PetersburgBlock:       chainCfg.PetersburgBlock,
		IstanbulBlock:         chainCfg.IstanbulBlock,
		MuirGlacierBlock:      chainCfg.MuirGlacierBlock,
		BerlinBlock:           chainCfg.BerlinBlock,
		LondonBlock:           chainCfg.LondonBlock,
		ArrowGlacierBlock:     chainCfg.ArrowGlacierBlock,
		GrayGlacierBlock:      chainCfg.GrayGlacierBlock,

		ShanghaiTime: shanghaiTime,
		CancunTime:   cancunTime,
	}
	vmConfig := vm.Config{
		NoBaseFee: true,
	}
	evm := vm.NewEVM(blkCtxE, txCtxE, intra, chainConfig, vmConfig)
	return evm, nil
}

func (backend *contractBackend) call(callMsg *ethereum.CallMsg, intra evmtypes.IntraBlockState) ([]byte, error) {
	evm, err := backend.prepare(intra)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare EVM for contract call")
	}
	t := time.Now()
	ret, gasLeft, err := evm.Call(vm.AccountRef(callMsg.From), erigonComm.Address(*callMsg.To), callMsg.Data, callMsg.Gas, uint256.MustFromBig(callMsg.Value), true)
	if err != nil {
		// Check if it's a revert error and extract the revert message
		if errors.Is(err, vm.ErrExecutionReverted) {
			revertMsg := iotexevm.ExtractRevertMessage(ret)
			log.L().Error("EVM call reverted",
				zap.String("from", callMsg.From.String()),
				zap.String("to", callMsg.To.String()),
				zap.Uint64("dataSize", uint64(len(callMsg.Data))),
				zap.String("revertMessage", revertMsg),
				zap.String("returnData", hex.EncodeToString(ret)),
			)
			return ret, errors.Wrapf(err, "execution reverted: %s", revertMsg)
		}
		return ret, errors.Wrapf(err, "error when system contract %x action mutates states", callMsg.To.Bytes())
	}
	log.L().Debug("EVM call result",
		zap.String("from", callMsg.From.String()),
		zap.String("to", callMsg.To.String()),
		zap.Uint64("dataSize", uint64(len(callMsg.Data))),
		zap.String("ret", hex.EncodeToString(ret)),
		zap.Uint64("gasUsed", callMsg.Gas-gasLeft),
		zap.Duration("duration", time.Since(t)),
	)
	return ret, nil
}

type intraStateReader struct {
	intra *erigonstate.IntraBlockState
	org   erigonstate.StateReader
}

func (sr *intraStateReader) ReadAccountData(address erigonComm.Address) (*erigonAcc.Account, error) {
	org, err := sr.org.ReadAccountData(address)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read account data for address %s", address.String())
	}
	acc := &erigonAcc.Account{
		Initialised:     false,
		Nonce:           sr.intra.GetNonce(address),
		Balance:         *sr.intra.GetBalance(address),
		Root:            erigonComm.Hash{},
		CodeHash:        sr.intra.GetCodeHash(address),
		Incarnation:     sr.intra.GetIncarnation(address),
		PrevIncarnation: 0,
	}
	if org != nil {
		acc.Initialised = org.Initialised
		acc.Root = org.Root
		acc.PrevIncarnation = org.PrevIncarnation
	}
	return acc, nil
}

func (sr *intraStateReader) ReadAccountStorage(address erigonComm.Address, incarnation uint64, key *erigonComm.Hash) ([]byte, error) {
	value := new(uint256.Int)
	sr.intra.GetState(address, key, value)
	return value.Bytes(), nil
}

func (sr *intraStateReader) ReadAccountCode(address erigonComm.Address, incarnation uint64, codeHash erigonComm.Hash) ([]byte, error) {
	code := sr.intra.GetCode(address)
	return code, nil
}
func (sr *intraStateReader) ReadAccountCodeSize(address erigonComm.Address, incarnation uint64, codeHash erigonComm.Hash) (int, error) {
	return len(sr.intra.GetCode(address)), nil
}

func (sr *intraStateReader) ReadAccountIncarnation(address erigonComm.Address) (uint64, error) {
	return sr.intra.GetIncarnation(address), nil
}
