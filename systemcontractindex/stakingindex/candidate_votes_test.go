package stakingindex

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

func TestCandidateVotes(t *testing.T) {
	require := require.New(t)
	g := genesis.TestDefault()
	t.Run("contract staking votes before Redsea", func(t *testing.T) {
		blkHeight := g.QuebecBlockHeight + 1
		ctx := protocol.WithBlockCtx(
			genesis.WithGenesisContext(context.Background(), g),
			protocol.BlockCtx{
				BlockHeight: blkHeight,
			},
		)
		ctx = protocol.WithFeatureCtx(ctx)
		csVotes := newCandidateVotes()
		cand := "candidate"
		csVotes.Add(cand, big.NewInt(0), big.NewInt(0))
		originCandVotes := csVotes.Votes(protocol.MustGetFeatureCtx(ctx), cand)
		if originCandVotes == nil {
			originCandVotes = big.NewInt(0)
		}
		csVotes.Add(cand, big.NewInt(100), big.NewInt(120))
		newVotes := csVotes.Votes(protocol.MustGetFeatureCtx(ctx), cand)
		if newVotes == nil {
			newVotes = big.NewInt(0)
		}
		require.EqualValues(100, newVotes.Sub(newVotes, originCandVotes).Uint64())
	})
	t.Run("contract staking votes after Redsea", func(t *testing.T) {
		blkHeight := g.RedseaBlockHeight
		ctx := protocol.WithBlockCtx(
			genesis.WithGenesisContext(context.Background(), g),
			protocol.BlockCtx{
				BlockHeight: blkHeight,
			},
		)
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		cand := "candidate"
		csVotes := newCandidateVotes()
		originCandVotes := csVotes.Votes(protocol.MustGetFeatureCtx(ctx), cand)
		if originCandVotes == nil {
			originCandVotes = big.NewInt(0)
		}
		csVotes.Add(cand, big.NewInt(100), big.NewInt(120))
		newVotes := csVotes.Votes(protocol.MustGetFeatureCtx(ctx), cand)
		if newVotes == nil {
			newVotes = big.NewInt(0)
		}
		require.EqualValues(120, newVotes.Sub(newVotes, originCandVotes).Uint64())
	})
}
