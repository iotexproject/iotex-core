package api

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/eth/tracers"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/v2/action"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/tracer"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

type (
	// CoreServiceReaderWithHeight is an interface for state reader at certain height
	CoreServiceReaderWithHeight interface {
		Account(address.Address) (*iotextypes.AccountMeta, *iotextypes.BlockIdentifier, error)
		PendingNonce(addr address.Address) (uint64, error)
		ReadContract(context.Context, address.Address, action.Envelope) (string, *iotextypes.Receipt, error)
		TraceCall(context.Context, address.Address, string, uint64, *big.Int, uint64, []byte,
			*tracers.TraceConfig) ([]byte, *action.Receipt, any, error)
	}

	coreServiceReaderWithHeight struct {
		cs     *coreService
		height uint64
	}
)

func newCoreServiceWithHeight(cs *coreService, height uint64) *coreServiceReaderWithHeight {
	return &coreServiceReaderWithHeight{
		cs:     cs,
		height: height,
	}
}

func (core *coreServiceReaderWithHeight) Account(addr address.Address) (*iotextypes.AccountMeta, *iotextypes.BlockIdentifier, error) {
	if !core.cs.archiveSupported {
		return nil, nil, ErrArchiveNotSupported
	}
	ctx, span := tracer.NewSpan(context.Background(), "coreServiceReaderWithHeight.Account")
	defer span.End()
	addrStr := addr.String()
	if addrStr == address.RewardingPoolAddr || addrStr == address.StakingBucketPoolAddr {
		return core.cs.getProtocolAccount(ctx, addrStr)
	}
	ctx = genesis.WithGenesisContext(ctx, core.cs.bc.Genesis())
	ws, err := core.cs.sf.WorkingSetAtHeight(ctx, core.height)
	if err != nil {
		return nil, nil, err
	}
	return core.cs.acccount(ctx, core.height, true, ws, addr)
}

func (core *coreServiceReaderWithHeight) ReadContract(ctx context.Context, callerAddr address.Address, elp action.Envelope) (string, *iotextypes.Receipt, error) {
	if !core.cs.archiveSupported {
		return "", nil, ErrArchiveNotSupported
	}
	log.Logger("api").Debug("receive read smart contract request")
	exec, ok := elp.Action().(*action.Execution)
	if !ok {
		return "", nil, status.Error(codes.InvalidArgument, "expecting action.Execution")
	}
	var (
		hdBytes = append(byteutil.Uint64ToBytesBigEndian(core.height), []byte(exec.Contract())...)
		key     = hash.Hash160b(append(hdBytes, exec.Data()...))
	)
	return core.cs.readContract(ctx, key, core.height, true, callerAddr, elp)
}

// TraceCall returns the trace result of call
func (core *coreServiceReaderWithHeight) TraceCall(ctx context.Context,
	callerAddr address.Address,
	contractAddress string,
	nonce uint64,
	amount *big.Int,
	gasLimit uint64,
	data []byte,
	config *tracers.TraceConfig) ([]byte, *action.Receipt, any, error) {
	var (
		g             = core.cs.bc.Genesis()
		blockGasLimit = g.BlockGasLimitByHeight(core.height)
	)
	if gasLimit == 0 {
		gasLimit = blockGasLimit
	}
	elp := (&action.EnvelopeBuilder{}).SetAction(action.NewExecution(contractAddress, amount, data)).
		SetGasLimit(gasLimit).Build()
	return core.cs.traceTx(ctx, new(tracers.Context), config, func(ctx context.Context) ([]byte, *action.Receipt, error) {
		return core.cs.simulateExecution(ctx, core.height, true, callerAddr, elp)
	})
}

func (core *coreServiceReaderWithHeight) PendingNonce(addr address.Address) (uint64, error) {
	var (
		g   = core.cs.bc.Genesis()
		ctx = genesis.WithGenesisContext(context.Background(), g)
	)
	ws, err := core.cs.sf.WorkingSetAtHeight(ctx, core.height)
	if err != nil {
		return 0, status.Error(codes.Internal, err.Error())
	}
	state, err := accountutil.AccountState(ctx, ws, addr)
	if err != nil {
		return 0, status.Error(codes.NotFound, err.Error())
	}
	if g.IsSumatra(core.height) {
		return state.PendingNonceConsideringFreshAccount(), nil
	}
	return state.PendingNonce(), nil
}
