package ioswarm

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	pb "github.com/iotexproject/iotex-core/v2/ioswarm/proto"
	"github.com/iotexproject/iotex-core/v2/state/factory"
)

// ActPoolAdapter wraps iotex-core's actpool.ActPool and blockchain.Blockchain
// to implement ActPoolReader for production use.
type ActPoolAdapter struct {
	pool   actpool.ActPool
	bc     blockchain.Blockchain
	logger *zap.Logger
}

// NewActPoolAdapter creates a new ActPoolAdapter.
func NewActPoolAdapter(pool actpool.ActPool, bc blockchain.Blockchain) *ActPoolAdapter {
	logger, _ := zap.NewProduction()
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ActPoolAdapter{pool: pool, bc: bc, logger: logger}
}

// PendingActions reads all pending transactions from the actpool and converts
// them to the ioswarm PendingTx format.
func (a *ActPoolAdapter) PendingActions() (txs []*PendingTx) {
	defer func() {
		if r := recover(); r != nil {
			a.logger.Error("panic in PendingActions", zap.Any("recover", r))
			txs = nil
		}
	}()
	actionMap := a.pool.PendingActionMap()
	for _, actions := range actionMap {
		for _, act := range actions {
			tx, err := sealedEnvelopeToTx(act)
			if err != nil {
				a.logger.Warn("failed to convert action to PendingTx", zap.Error(err))
				continue
			}
			txs = append(txs, tx)
		}
	}
	return txs
}

// BlockHeight returns the current tip block height.
func (a *ActPoolAdapter) BlockHeight() uint64 {
	return a.bc.TipHeight()
}

// sealedEnvelopeToTx converts a SealedEnvelope to a PendingTx.
func sealedEnvelopeToTx(act *action.SealedEnvelope) (*PendingTx, error) {
	h, err := act.Hash()
	if err != nil {
		return nil, err
	}

	dest, _ := act.Destination()

	// Extract amount via type assertion on the underlying action
	amount := "0"
	if afc, ok := act.Action().(interface{ Amount() *big.Int }); ok {
		if a := afc.Amount(); a != nil {
			amount = a.String()
		}
	}

	raw, err := proto.Marshal(act.Proto())
	if err != nil {
		return nil, err
	}

	return &PendingTx{
		Hash:     hex.EncodeToString(h[:]),
		From:     act.SenderAddress().String(),
		To:       dest,
		Nonce:    act.Nonce(),
		Amount:   amount,
		GasLimit: act.Gas(),
		GasPrice: act.GasPrice().String(),
		Data:     act.Data(),
		RawBytes: raw,
	}, nil
}

// StateReaderAdapter wraps iotex-core's factory.Factory to implement StateReader
// for production use.
type StateReaderAdapter struct {
	sf      factory.Factory
	bc      blockchain.Blockchain
	genesis genesis.Genesis
	logger  *zap.Logger
}

// NewStateReaderAdapter creates a new StateReaderAdapter.
func NewStateReaderAdapter(sf factory.Factory, bc blockchain.Blockchain, g genesis.Genesis) *StateReaderAdapter {
	logger, _ := zap.NewProduction()
	if logger == nil {
		logger = zap.NewNop()
	}
	return &StateReaderAdapter{sf: sf, bc: bc, genesis: g, logger: logger}
}

// chainCtx returns a context with both genesis and blockchain info attached
// (required by WorkingSet and EVM operations).
func (s *StateReaderAdapter) chainCtx() context.Context {
	ctx := genesis.WithGenesisContext(context.Background(), s.genesis)
	return protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
		Tip: protocol.TipInfo{
			Height: s.bc.TipHeight(),
		},
		ChainID:      s.bc.ChainID(),
		EvmNetworkID: s.bc.EvmNetworkID(),
	})
}

// AccountState reads the confirmed account state from the stateDB.
func (s *StateReaderAdapter) AccountState(addr string) (snap *pb.AccountSnapshot, err error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("panic in AccountState", zap.String("addr", addr), zap.Any("recover", r))
			snap, err = nil, fmt.Errorf("AccountState panic: %v", r)
		}
	}()
	ioAddr, err := address.FromString(addr)
	if err != nil {
		return nil, err
	}
	acct, err := accountutil.AccountState(context.Background(), s.sf, ioAddr)
	if err != nil {
		return nil, err
	}
	return &pb.AccountSnapshot{
		Address:  addr,
		Balance:  acct.Balance.String(),
		Nonce:    acct.PendingNonce(),
		CodeHash: acct.CodeHash,
	}, nil
}

// GetCode returns contract bytecode from the stateDB.
func (s *StateReaderAdapter) GetCode(addr string) (code []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("panic in GetCode", zap.String("addr", addr), zap.Any("recover", r))
			code, err = nil, fmt.Errorf("GetCode panic: %v", r)
		}
	}()
	ioAddr, err := address.FromString(addr)
	if err != nil {
		return nil, err
	}
	ctx := s.chainCtx()
	ws, err := s.sf.WorkingSet(ctx)
	if err != nil {
		return nil, err
	}
	defer ws.Close()
	return evm.ReadContractCode(ctx, ws, ioAddr)
}

// GetStorageAt returns a storage slot value from the stateDB.
// slot is a hex-encoded 32-byte key (e.g. "0x0000...0000").
func (s *StateReaderAdapter) GetStorageAt(addr, slot string) (val string, err error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("panic in GetStorageAt", zap.String("addr", addr), zap.Any("recover", r))
			val, err = "", fmt.Errorf("GetStorageAt panic: %v", r)
		}
	}()
	ioAddr, err := address.FromString(addr)
	if err != nil {
		return "", err
	}
	slotHex := slot
	if len(slotHex) > 2 && slotHex[:2] == "0x" {
		slotHex = slotHex[2:]
	}
	key, err := hex.DecodeString(slotHex)
	if err != nil {
		return "", err
	}
	ctx := s.chainCtx()
	ws, err := s.sf.WorkingSet(ctx)
	if err != nil {
		return "", err
	}
	defer ws.Close()
	raw, err := evm.ReadContractStorage(ctx, ws, ioAddr, key)
	if err != nil {
		return "", err
	}
	return "0x" + hex.EncodeToString(raw), nil
}
