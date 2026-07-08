package stakingindex

import (
	"context"
	"fmt"
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

func TestCandidateVotesInterface(t *testing.T) {
	r := require.New(t)

	cvs := []CandidateVotes{
		newCandidateVotesWithBuffer(newCandidateVotes()),
		newCandidateVotesWrapper(newCandidateVotesWithBuffer(newCandidateVotes())),
		newCandidateVotesWrapperCommitInClone(newCandidateVotesWithBuffer(newCandidateVotes())),
	}
	g := genesis.TestDefault()
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctxBeforeRedsea := protocol.MustGetFeatureCtx(protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: g.RedseaBlockHeight - 1})))
	ctxAfterRedsea := protocol.MustGetFeatureCtx(protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: g.RedseaBlockHeight})))
	for _, cv := range cvs {
		t.Run(fmt.Sprintf("%T", cv), func(t *testing.T) {
			// not exist candidate
			r.Nil(cv.Votes(ctxAfterRedsea, "notexist"))
			r.Nil(cv.Votes(ctxBeforeRedsea, "notexist"))
			r.False(cv.IsDirty())

			// add candidate
			adds := []struct {
				cand   string
				amount string
				votes  string
			}{
				{"candidate1", "100", "120"},
				{"candidate2", "200", "240"},
				{"candidate1", "300", "360"},
				{"candidate3", "400", "480"},
				{"candidate2", "500", "600"},
				{"candidate2", "-200", "-240"},
			}
			expects := []struct {
				cand   string
				amount string
				votes  string
			}{
				{"candidate1", "400", "480"},
				{"candidate2", "500", "600"},
				{"candidate3", "400", "480"},
			}
			for _, add := range adds {
				amount := big.NewInt(0)
				votes := big.NewInt(0)
				_, ok := amount.SetString(add.amount, 10)
				r.True(ok)
				_, ok = votes.SetString(add.votes, 10)
				r.True(ok)
				cv.Add(add.cand, amount, votes)
			}
			checkVotes := func(cv CandidateVotes) {
				for _, expect := range expects {
					amount := big.NewInt(0)
					votes := big.NewInt(0)
					_, ok := amount.SetString(expect.amount, 10)
					r.True(ok)
					_, ok = votes.SetString(expect.votes, 10)
					r.True(ok)
					r.Equal(amount, cv.Votes(ctxBeforeRedsea, expect.cand))
					r.Equal(votes, cv.Votes(ctxAfterRedsea, expect.cand))
				}
			}
			checkVotes(cv)
			cl := cv.Clone()
			checkVotes(cl)
			// both cv and cl are dirty
			r.True(cv.IsDirty())
			r.True(cl.IsDirty())
			// serialize dirty cv should fail
			_, err := cv.Serialize()
			r.ErrorIs(err, ErrCandidateVotesIsDirty)
			// commit cv
			reset := cv.Commit()
			r.False(reset.IsDirty())
			data, err := reset.Serialize()
			r.NoError(err)
			// deserialize to new cv
			decv := newCandidateVotes()
			err = decv.Deserialize(data)
			r.NoError(err)
			checkVotes(newCandidateVotesWithBuffer(decv))
			// adds not affect base
			cv.Add("candidate4", big.NewInt(1000), big.NewInt(1200))
			cv.Add("candidate1", big.NewInt(-100), big.NewInt(-120))
			cv.Add("candidate2", big.NewInt(100), big.NewInt(120))
			checkVotes(cv.Base())
		})
	}
}

func TestCandidateVotesWrapper(t *testing.T) {
	r := require.New(t)
	baseCv := newCandidateVotesWithBuffer(newCandidateVotes())
	baseCv.Add("candidate1", big.NewInt(100), big.NewInt(120))
	baseCv.Add("candidate2", big.NewInt(200), big.NewInt(240))
	baseCv.Add("candidate3", big.NewInt(400), big.NewInt(480))
	base := baseCv.Commit()
	// wrap's changes should not affect base
	wrap := newCandidateVotesWrapper(base)
	wrap.Add("candidate1", big.NewInt(300), big.NewInt(360))
	wrap.Add("candidate4", big.NewInt(1000), big.NewInt(1200))
	candidateVotesEqual(r, base, wrap.Base(), []string{"candidate1", "candidate2", "candidate3", "candidate4"})
	// multiple wraps return base recursively
	wrap2 := newCandidateVotesWrapper(wrap)
	wrap2.Add("candidate2", big.NewInt(500), big.NewInt(600))
	wrap2.Add("candidate5", big.NewInt(2000), big.NewInt(2400))
	candidateVotesEqual(r, base, wrap2.Base(), []string{"candidate1", "candidate2", "candidate3", "candidate4", "candidate5"})
	// commit wrap should apply all changes to base
	wrap2.Commit()
	candidateVotesEqual(r, base, wrap2, []string{"candidate1", "candidate2", "candidate3", "candidate4", "candidate5"})
}

func TestCandidateVotesWrapperCommitInClone(t *testing.T) {
	r := require.New(t)
	baseCv := newCandidateVotesWithBuffer(newCandidateVotes())
	baseCv.Add("candidate1", big.NewInt(100), big.NewInt(120))
	baseCv.Add("candidate2", big.NewInt(200), big.NewInt(240))
	baseCv.Add("candidate3", big.NewInt(400), big.NewInt(480))
	base := baseCv.Commit()
	wrap := newCandidateVotesWrapper(base)
	wrap.Add("candidate1", big.NewInt(300), big.NewInt(360))
	wrap.Add("candidate4", big.NewInt(1000), big.NewInt(1200))
	wrap2 := newCandidateVotesWrapper(wrap)
	wrap2.Add("candidate2", big.NewInt(500), big.NewInt(600))
	wrap2.Add("candidate5", big.NewInt(2000), big.NewInt(2400))
	wrap3 := newCandidateVotesWrapper(wrap)
	wrap3.Add("candidate3", big.NewInt(700), big.NewInt(840))
	wrap3.Add("candidate1", big.NewInt(-3000), big.NewInt(-3600))
	wrap3Clone := wrap3.Clone()

	// base return the first base
	wrap4 := newCandidateVotesWrapperCommitInClone(wrap3)
	wrap4.Add("candidate2", big.NewInt(3000), big.NewInt(3600))
	wrap4.Add("candidate3", big.NewInt(4000), big.NewInt(4800))
	candidateVotesEqual(r, wrap4.Base(), wrap3, []string{"candidate1", "candidate2", "candidate3", "candidate4", "candidate5"})
	// commit wrap4 should not apply all changes to base
	wrap4.Commit()
	candidateVotesEqual(r, wrap3, wrap3Clone, []string{"candidate1", "candidate2", "candidate3", "candidate4", "candidate5"})
}

func candidateVotesEqual(r *require.Assertions, cv1, cv2 CandidateVotes, cands []string) {
	g := genesis.TestDefault()
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctxBeforeRedsea := protocol.MustGetFeatureCtx(protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: g.RedseaBlockHeight - 1})))
	ctxAfterRedsea := protocol.MustGetFeatureCtx(protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: g.RedseaBlockHeight})))
	for _, cand := range cands {
		r.Equal(cv1.Votes(ctxBeforeRedsea, cand), cv2.Votes(ctxBeforeRedsea, cand))
		r.Equal(cv1.Votes(ctxAfterRedsea, cand), cv2.Votes(ctxAfterRedsea, cand))
	}
}
