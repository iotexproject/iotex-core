package api

import (
	"context"
	"fmt"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/tracer"
)

type (
	CoreServiceReaderWithHeight interface {
		Account(addr address.Address) (*iotextypes.AccountMeta, *iotextypes.BlockIdentifier, error)
		PendingNonce(addr address.Address) (uint64, error)
		ReadContractStorage(context.Context, address.Address, []byte) ([]byte, error)
		EstimateExecutionGasConsumption(context.Context, action.Envelope, address.Address) (uint64, []byte, error)
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
	ctx, span := tracer.NewSpan(context.Background(), "coreService.Account")
	defer span.End()
	addrStr := addr.String()
	if addrStr == address.RewardingPoolAddr || addrStr == address.StakingBucketPoolAddr {
		return core.getProtocolAccount(ctx, addrStr)
	}
	return core.cs.account(ctx, newStateReaderWithHeight(core.cs.sf, core.height), addr)
}

func (core *coreServiceReaderWithHeight) getProtocolAccount(ctx context.Context, addr string) (*iotextypes.AccountMeta, *iotextypes.BlockIdentifier, error) {
	span := tracer.SpanFromContext(ctx)
	defer span.End()
	var (
		balance string
		out     *iotexapi.ReadStateResponse
		err     error
	)
	heightStr := fmt.Sprintf("%d", core.height)
	switch addr {
	case address.RewardingPoolAddr:
		if out, err = core.cs.ReadState("rewarding", heightStr, []byte("TotalBalance"), nil); err != nil {
			return nil, nil, err
		}
		val, ok := new(big.Int).SetString(string(out.GetData()), 10)
		if !ok {
			return nil, nil, errors.New("balance convert error")
		}
		balance = val.String()
	case address.StakingBucketPoolAddr:
		methodName, err := proto.Marshal(&iotexapi.ReadStakingDataMethod{
			Method: iotexapi.ReadStakingDataMethod_TOTAL_STAKING_AMOUNT,
		})
		if err != nil {
			return nil, nil, err
		}
		arg, err := proto.Marshal(&iotexapi.ReadStakingDataRequest{
			Request: &iotexapi.ReadStakingDataRequest_TotalStakingAmount_{
				TotalStakingAmount: &iotexapi.ReadStakingDataRequest_TotalStakingAmount{},
			},
		})
		if err != nil {
			return nil, nil, err
		}
		if out, err = core.cs.ReadState("staking", heightStr, methodName, [][]byte{arg}); err != nil {
			return nil, nil, err
		}
		acc := iotextypes.AccountMeta{}
		if err := proto.Unmarshal(out.GetData(), &acc); err != nil {
			return nil, nil, errors.Wrap(err, "failed to unmarshal account meta")
		}
		balance = acc.GetBalance()
	default:
		return nil, nil, errors.Errorf("invalid address %s", addr)
	}
	return &iotextypes.AccountMeta{
		Address: addr,
		Balance: balance,
	}, out.GetBlockIdentifier(), nil
}

func (core *coreServiceReaderWithHeight) PendingNonce(addr address.Address) (uint64, error) {
	var (
		g   = core.cs.bc.Genesis()
		ctx = genesis.WithGenesisContext(context.Background(), g)
	)
	state, err := accountutil.AccountState(ctx, newStateReaderWithHeight(core.cs.sf, core.height), addr)
	if err != nil {
		return 0, status.Error(codes.NotFound, err.Error())
	}
	if g.IsSumatra(core.height) {
		return state.PendingNonceConsideringFreshAccount(), nil
	}
	return state.PendingNonce(), nil
}

// ReadContractStorage reads contract's storage
func (core *coreServiceReaderWithHeight) ReadContractStorage(ctx context.Context, addr address.Address, key []byte) ([]byte, error) {
	ctx, err := core.cs.bc.Context(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return core.cs.sf.ReadContractStorageAtHeight(ctx, core.height, addr, key)
}

func (core *coreServiceReaderWithHeight) EstimateExecutionGasConsumption(ctx context.Context, elp action.Envelope, callerAddr address.Address) (uint64, []byte, error) {
	return core.cs.estimateExecutionGasConsumption(ctx, core.height, true, elp, callerAddr)
}
