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
	erigonAcc "github.com/erigontech/erigon/core/types/accounts"
	"github.com/erigontech/erigon/core/vm"
	"github.com/erigontech/erigon/core/vm/evmtypes"
	"github.com/ethereum/go-ethereum"
	"github.com/holiman/uint256"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	iotexevm "github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type (
	contractBacked struct {
		intraBlockState *erigonstate.IntraBlockState
		org             erigonstate.StateReader

		// helper fields
		height       uint64
		timestamp    time.Time
		g            *genesis.Genesis
		evmNetworkID uint32
	}
)

func NewContractBackend(intraBlockState *erigonstate.IntraBlockState, org erigonstate.StateReader, height uint64, timestamp time.Time, g *genesis.Genesis, evmNetworkID uint32) *contractBacked {
	return &contractBacked{
		intraBlockState: intraBlockState,
		org:             org,
		height:          height,
		timestamp:       timestamp,
		g:               g,
		evmNetworkID:    evmNetworkID,
	}
}

func (backend *contractBacked) Call(callMsg *ethereum.CallMsg) ([]byte, error) {
	return backend.call(callMsg, erigonstate.New(&intraStateReader{backend.intraBlockState, backend.org}))
}

func (backend *contractBacked) Handle(callMsg *ethereum.CallMsg) error {
	_, err := backend.call(callMsg, backend.intraBlockState)
	return err
}

func (backend *contractBacked) Deploy(callMsg *ethereum.CallMsg) (address.Address, error) {
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
	log.L().Info("EVM deployment result",
		zap.String("from", callMsg.From.String()),
		zap.String("data", hex.EncodeToString(callMsg.Data)),
		zap.String("ret", hex.EncodeToString(ret)),
		zap.String("address", addr.String()),
		zap.Uint64("gasLeft", leftGas),
	)

	return address.FromBytes(addr.Bytes())
}

func (backend *contractBacked) Exists(addr address.Address) bool {
	return backend.intraBlockState.Exist(erigonComm.BytesToAddress(addr.Bytes()))
}

func (backend *contractBacked) prepare(intra evmtypes.IntraBlockState) (*vm.EVM, error) {

	// deploy system contracts
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
		Coinbase:    erigonComm.Address{},
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

func (backend *contractBacked) call(callMsg *ethereum.CallMsg, intra evmtypes.IntraBlockState) ([]byte, error) {
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
	log.L().Info("EVM call result",
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
