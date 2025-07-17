package factory

import (
	"context"
	"encoding/hex"
	"math/big"

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

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type contractBacked struct {
	ws  *workingSet
	reg *protocol.Registry
	ctx context.Context
}

func newContractBacked(ctx context.Context, ws *workingSet, reg *protocol.Registry) *contractBacked {
	return &contractBacked{
		ws:  ws,
		reg: reg,
		ctx: ctx,
	}
}

func (backend *contractBacked) Call(callMsg *ethereum.CallMsg) ([]byte, error) {
	intra, _ := backend.ws.Erigon()
	if intra == nil {
		return nil, errors.New("intra block state is not available")
	}
	return backend.call(callMsg, erigonstate.New(&intraStateReader{intra}))
}

func (backend *contractBacked) Handle(callMsg *ethereum.CallMsg) error {
	intra, _ := backend.ws.Erigon()
	if intra == nil {
		return errors.New("intra block state is not available")
	}
	_, err := backend.call(callMsg, intra)
	return err
}

func (backend *contractBacked) call(callMsg *ethereum.CallMsg, intra evmtypes.IntraBlockState) ([]byte, error) {
	ctx := backend.ctx
	blkCtx := protocol.MustGetBlockCtx(ctx)

	// deploy system contracts
	blkCtxE := evmtypes.BlockContext{
		CanTransfer: func(state evmtypes.IntraBlockState, addr erigonComm.Address, amount *uint256.Int) bool {
			log.L().Info("CanTransfer called in erigon genesis state creation",
				zap.String("address", addr.String()),
				zap.String("amount", amount.String()),
			)
			return true
		},
		Transfer: func(state evmtypes.IntraBlockState, from erigonComm.Address, to erigonComm.Address, amount *uint256.Int, bailout bool) {
			log.L().Info("Transfer called in erigon genesis state creation",
				zap.String("from", from.String()),
				zap.String("to", to.String()),
				zap.String("amount", amount.String()),
			)
			return
		},
		GetHash: func(block uint64) erigonComm.Hash {
			log.L().Info("GetHash called in erigon genesis state creation",
				zap.Uint64("block", block),
			)
			return erigonComm.Hash{}
		},
		PostApplyMessage: func(ibs evmtypes.IntraBlockState, sender erigonComm.Address, coinbase erigonComm.Address, result *evmtypes.ExecutionResult) {
			log.L().Info("PostApplyMessage called in erigon genesis state creation",
				zap.String("sender", sender.String()),
				zap.String("coinbase", coinbase.String()),
			)
			return
		},
		Coinbase:    erigonComm.Address{},
		GasLimit:    100000000,
		MaxGasLimit: true,
		BlockNumber: blkCtx.BlockHeight,
		Time:        uint64(blkCtx.BlockTimeStamp.Unix()),
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
		HomesteadBlock:        chainCfg.HomesteadBlock,
		DAOForkBlock:          chainCfg.DAOForkBlock,
		TangerineWhistleBlock: chainCfg.EIP150Block,
		SpuriousDragonBlock:   chainCfg.EIP155Block,
		ByzantiumBlock:        chainCfg.ByzantiumBlock,
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
	ret, gasLeft, err := evm.Call(vm.AccountRef(callMsg.From), erigonComm.Address(*callMsg.To), callMsg.Data, 1000000, uint256.NewInt(0), true)
	if err != nil {
		return nil, errors.Wrapf(err, "error when system contract %x action mutates states", callMsg.To.Bytes())
	}
	log.L().Info("EVM call result",
		zap.String("from", callMsg.From.String()),
		zap.String("to", callMsg.To.String()),
		zap.String("data", hex.EncodeToString(callMsg.Data)),
		zap.String("ret", hex.EncodeToString(ret)),
		zap.Uint64("gasLeft", gasLeft),
	)
	return ret, nil
}

type intraStateReader struct {
	intra *erigonstate.IntraBlockState
}

func (sr *intraStateReader) ReadAccountData(address erigonComm.Address) (*erigonAcc.Account, error) {
	acc := &erigonAcc.Account{
		Initialised:     true,
		Nonce:           sr.intra.GetNonce(address),
		Balance:         *sr.intra.GetBalance(address),
		Root:            erigonComm.Hash{},
		CodeHash:        sr.intra.GetCodeHash(address),
		Incarnation:     sr.intra.GetIncarnation(address),
		PrevIncarnation: 0,
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
