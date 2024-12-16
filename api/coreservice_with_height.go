package api

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/tracer"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

type (
	// CoreServiceReaderWithHeight is an interface for state reader at certain height
	CoreServiceReaderWithHeight interface {
		Account(address.Address) (*iotextypes.AccountMeta, *iotextypes.BlockIdentifier, error)
		ReadContract(context.Context, address.Address, action.Envelope) (string, *iotextypes.Receipt, error)
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
