// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package execution

import (
	"context"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/params"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

const (
	// the maximum size of execution allowed
	_executionSizeLimit48KB = uint32(48 * 1024)
	_executionSizeLimit32KB = uint32(32 * 1024)
	// TODO: it works only for one instance per protocol definition now
	_protocolID = "smart_contract"
)

// System contracts.
var (
	// SystemAddress is where the system-transaction is sent from as per EIP-4788
	SystemAddress = params.SystemAddress

	// EIP-2935 - Serve historical block hashes from state
	HistoryStorageAddress = params.HistoryStorageAddress
	HistoryStorageCode    = params.HistoryStorageCode
)

// Protocol defines the protocol of handling executions
type Protocol struct {
	depositGas protocol.DepositGas
	addr       address.Address
}

// NewProtocol instantiates the protocol of exeuction
// TODO: remove unused getBlockHash and getBlockTime
func NewProtocol(_ evm.GetBlockHash, depositGas protocol.DepositGas, _ evm.GetBlockTime) *Protocol {
	h := hash.Hash160b([]byte(_protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of vote protocol", zap.Error(err))
	}
	return &Protocol{depositGas: depositGas, addr: addr}
}

// FindProtocol finds the registered protocol from registry
func FindProtocol(registry *protocol.Registry) *Protocol {
	if registry == nil {
		return nil
	}
	p, ok := registry.Find(_protocolID)
	if !ok {
		return nil
	}
	ep, ok := p.(*Protocol)
	if !ok {
		log.S().Panic("fail to cast execution protocol")
	}
	return ep
}

// CreatePreStates creates states for state manager
func (p *Protocol) CreatePreStates(ctx context.Context, sm protocol.StateManager) error {
	g := genesis.MustExtractGenesisContext(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)
	fCtx := protocol.MustGetFeatureCtx(ctx)
	if fCtx.PrePectraEVM {
		return nil
	}
	// deploy the history storage contract at block
	if blkCtx.BlockHeight == g.ToBeEnabledBlockHeight {
		if err := p.deployHistoryBlockStorageContract(ctx, sm); err != nil {
			return err
		}
	}
	// set the block hash of previous block
	return p.setPreviousBlockHash(ctx, sm)
}

func (p *Protocol) systemCallContext(ctx context.Context, sm protocol.StateManager) (context.Context, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	systemAddr, err := address.FromBytes(SystemAddress.Bytes())
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse system address")
	}
	acc, err := accountutil.LoadAccount(sm, systemAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load system account")
	}
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
		Caller:   systemAddr,
		Nonce:    acc.PendingNonceConsideringFreshAccount(),
		GasPrice: big.NewInt(0),
	})
	blkCtx.BaseFee = nil
	blkCtx.GasLimit = math.MaxUint64
	ctx = protocol.WithBlockCtx(ctx, blkCtx)
	ctx = evm.WithHelperCtx(ctx, evm.HelperContext{
		DepositGasFunc: func(ctx context.Context, sm protocol.StateManager, i *big.Int, do ...protocol.DepositOption) ([]*action.TransactionLog, error) {
			return nil, nil
		},
		GetBlockHash: bcCtx.GetBlockHash,
		GetBlockTime: bcCtx.GetBlockTime,
	})
	return ctx, nil
}

func (p *Protocol) deployHistoryBlockStorageContract(ctx context.Context, sm protocol.StateManager) error {
	sctx, err := p.systemCallContext(ctx, sm)
	if err != nil {
		return errors.Wrap(err, "failed to create system call context")
	}
	contractAddr, err := address.FromBytes(HistoryStorageAddress.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to parse contract address")
	}
	if err := evm.DeploySystemContracts(sctx, sm,
		[]address.Address{contractAddr},
		[][]byte{HistoryStorageCode},
	); err != nil {
		return errors.Wrap(err, "failed to deploy history storage contract")
	}
	log.L().Info("Deployed history block storage contract", zap.String("address", contractAddr.String()))
	return nil
}

func (p *Protocol) setPreviousBlockHash(ctx context.Context, sm protocol.StateManager) error {
	sctx, err := p.systemCallContext(ctx, sm)
	if err != nil {
		return errors.Wrap(err, "failed to create system call context")
	}
	contractAddr, err := address.FromBytes(HistoryStorageAddress.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to parse contract address")
	}
	bcCtx := protocol.MustGetBlockchainCtx(sctx)
	actCtx := protocol.MustGetActionCtx(sctx)

	// construct the envelope and execute the transaction
	elp := action.NewEnvelope(action.NewLegacyTx(bcCtx.ChainID, actCtx.Nonce, 30_000_000, actCtx.GasPrice),
		action.NewExecution(contractAddr.String(), big.NewInt(0), bcCtx.Tip.Hash[:]))
	output, err := evm.HandleSystemContractCall(sctx, sm, elp)
	if err != nil {
		return err
	}
	log.L().Debug("Set previous block hash", log.Hex("hash", bcCtx.Tip.Hash[:]), log.Hex("output", output))
	return nil
}

// Handle handles an execution
func (p *Protocol) Handle(ctx context.Context, elp action.Envelope, sm protocol.StateManager) (*action.Receipt, error) {
	if _, ok := elp.Action().(*action.Execution); !ok {
		return nil, nil
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	ctx = evm.WithHelperCtx(ctx, evm.HelperContext{
		GetBlockHash:   bcCtx.GetBlockHash,
		GetBlockTime:   bcCtx.GetBlockTime,
		DepositGasFunc: p.depositGas,
	})
	_, receipt, err := evm.ExecuteContract(ctx, sm, elp)

	if err != nil {
		return nil, errors.Wrap(err, "failed to execute contract")
	}
	return receipt, nil
}

// Validate validates an execution
func (p *Protocol) Validate(ctx context.Context, elp action.Envelope, _ protocol.StateReader) error {
	exec, ok := elp.Action().(*action.Execution)
	if !ok {
		return nil
	}
	var (
		sizeLimit = _executionSizeLimit48KB
		dataSize  = uint32(len(exec.Data()))
	)
	fCtx := protocol.MustGetFeatureCtx(ctx)
	if fCtx.ExecutionSizeLimit32KB {
		sizeLimit = _executionSizeLimit32KB
		dataSize = elp.Size()
	}

	// Reject oversize execution
	if dataSize > sizeLimit {
		return action.ErrOversizedData
	}
	if len(elp.BlobHashes()) > 0 && elp.To() == nil {
		return errors.New("cannot create contract in blob tx")
	}
	// Reject SetCodeTxType before Pectra EVM upgrade
	if fCtx.PrePectraEVM && elp.TxType() == action.SetCodeTxType {
		return errors.Wrapf(action.ErrInvalidAct, "SetCodeTxType is not allowed before Pectra EVM upgrade")
	}
	return nil
}

// ReadState read the state on blockchain via protocol
func (p *Protocol) ReadState(context.Context, protocol.StateReader, []byte, ...[]byte) ([]byte, uint64, error) {
	return nil, uint64(0), protocol.ErrUnimplemented
}

// Register registers the protocol with a unique ID
func (p *Protocol) Register(r *protocol.Registry) error {
	return r.Register(_protocolID, p)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (p *Protocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(_protocolID, p)
}

// Name returns the name of protocol
func (p *Protocol) Name() string {
	return _protocolID
}
