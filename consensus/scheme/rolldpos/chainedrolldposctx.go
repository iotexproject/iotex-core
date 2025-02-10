package rolldpos

import (
	"go.uber.org/zap"
)

type chainedRollDPoSCtx struct {
	*rollDPoSCtx
}

func NewChainedRollDPoSCtx(
	ctx *rollDPoSCtx,
) (RDPoSCtx, error) {
	return &chainedRollDPoSCtx{
		rollDPoSCtx: ctx,
	}, nil
}

func (cctx *chainedRollDPoSCtx) Prepare() error {
	ctx := cctx.rollDPoSCtx
	ctx.logger().Debug("prepare")
	ctx.mutex.Lock()
	ctx.logger().Debug("prepare lock")
	defer ctx.mutex.Unlock()
	height := cctx.chain.DraftHeight() + 1
	ctx.logger().Debug("prepare draft height", zap.Uint64("height", height))
	newRound, err := ctx.roundCalc.UpdateRound(ctx.round, height, ctx.BlockInterval(height), ctx.clock.Now(), ctx.toleratedOvertime)
	if err != nil {
		return err
	}
	ctx.logger().Debug(
		"new round",
		zap.Uint64("newheight", newRound.height),
		zap.String("newts", ctx.clock.Now().String()),
		zap.Uint64("newepoch", newRound.epochNum),
		zap.Uint64("newepochStartHeight", newRound.epochStartHeight),
		zap.Uint32("newround", newRound.roundNum),
		zap.String("newroundStartTime", newRound.roundStartTime.String()),
	)
	ctx.round = newRound
	_consensusHeightMtc.WithLabelValues().Set(float64(ctx.round.height))
	_timeSlotMtc.WithLabelValues().Set(float64(ctx.round.roundNum))
	return nil
}
