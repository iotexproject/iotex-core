package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/tracer"
)

type (
	CoreServiceReaderWithHeight interface {
		Account(addr address.Address) (*iotextypes.AccountMeta, *iotextypes.BlockIdentifier, error)
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
		return core.cs.getProtocolAccount(ctx, addrStr)
	}
	span.AddEvent("accountutil.AccountStateWithHeight")
	ctx = genesis.WithGenesisContext(ctx, core.cs.bc.Genesis())
	stateReader := newStateReaderWithHeight(core.cs.sf, core.height)
	state, err := accountutil.AccountState(ctx, stateReader, addr)
	if err != nil {
		return nil, nil, status.Error(codes.NotFound, err.Error())
	}
	var pendingNonce uint64
	ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: core.height,
	}))
	if protocol.MustGetFeatureCtx(ctx).RefactorFreshAccountConversion {
		pendingNonce = state.PendingNonceConsideringFreshAccount()
	} else {
		pendingNonce = state.PendingNonce()
	}
	span.AddEvent("indexer.GetActionCount")
	// TODO: get action count from indexer
	numActions := uint64(0)
	// numActions, err := core.cs.indexer.GetActionCountByAddress(hash.BytesToHash160(addr.Bytes()))
	// if err != nil {
	// 	return nil, nil, status.Error(codes.NotFound, err.Error())
	// }
	// TODO: deprecate nonce field in account meta
	accountMeta := &iotextypes.AccountMeta{
		Address:    addrStr,
		Balance:    state.Balance.String(),
		Nonce:      pendingNonce,
		NumActions: numActions,
		IsContract: state.IsContract(),
	}
	if state.IsContract() {
		var code protocol.SerializableBytes
		_, err = stateReader.State(&code, protocol.NamespaceOption(evm.CodeKVNameSpace), protocol.KeyOption(state.CodeHash))
		if err != nil {
			return nil, nil, status.Error(codes.NotFound, err.Error())
		}
		accountMeta.ContractByteCode = code
	}
	span.AddEvent("bc.BlockHeaderByHeight")
	header, err := core.cs.bc.BlockHeaderByHeight(core.height)
	if err != nil {
		return nil, nil, status.Error(codes.NotFound, err.Error())
	}
	hash := header.HashBlock()
	span.AddEvent("coreService.Account.End")
	return accountMeta, &iotextypes.BlockIdentifier{
		Hash:   hex.EncodeToString(hash[:]),
		Height: core.height,
	}, nil
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
