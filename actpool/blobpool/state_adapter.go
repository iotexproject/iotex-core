package blobpool

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/log"
)

type stateAdapter struct {
	g  *genesis.Genesis
	sf protocol.StateReader
}

func newStateAdapter(g *genesis.Genesis, sf protocol.StateReader) *stateAdapter {
	return &stateAdapter{
		g:  g,
		sf: sf,
	}
}

func (s *stateAdapter) context(ctx context.Context) context.Context {
	height, _ := s.sf.Height()
	return protocol.WithFeatureCtx(protocol.WithBlockCtx(
		genesis.WithGenesisContext(ctx, *s.g), protocol.BlockCtx{
			BlockHeight: height + 1,
		}))
}

func (s *stateAdapter) GetNonce(addr common.Address) uint64 {
	ctx := s.context(context.Background())
	confirmedState, err := accountutil.AccountState(ctx, s.sf, addr)
	if err != nil {
		log.L().Error("failed to get account state", zap.Error(err))
		return 0
	}
	if protocol.MustGetFeatureCtx(ctx).UseZeroNonceForFreshAccount {
		return confirmedState.PendingNonceConsideringFreshAccount()
	}
	return confirmedState.PendingNonce()
}

func (s *stateAdapter) GetBalance(addr common.Address) *uint256.Int {
	balance := uint256.NewInt(0)
	ctx := s.context(context.Background())
	confirmedState, err := accountutil.AccountState(ctx, s.sf, addr)
	if err != nil {
		log.L().Error("failed to get account state", zap.Error(err))
		return balance
	}
	balance.SetFromBig(confirmedState.Balance)
	return balance
}
