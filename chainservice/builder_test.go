package chainservice

import (
	"testing"
	"time"

	"github.com/mohae/deepcopy"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestEstimateTipHeight(t *testing.T) {
	r := require.New(t)
	cfg := deepcopy.Copy(config.Default).(config.Config)
	sk := identityset.PrivateKey(1)
	t.Run("after dardanelles", func(t *testing.T) {
		blk, err := block.NewBuilder(block.RunnableActions{}).SetHeight(cfg.Genesis.DardanellesBlockHeight + 1).SignAndBuild(sk)
		r.NoError(err)
		r.Equal(blk.Height()+2, estimateTipHeight(&cfg, &blk, 10*time.Second))
	})
	t.Run("before dardanelles", func(t *testing.T) {
		blk, err := block.NewBuilder(block.RunnableActions{}).SetHeight(cfg.Genesis.DardanellesBlockHeight - 100).SignAndBuild(sk)
		r.NoError(err)
		r.Equal(blk.Height()+1, estimateTipHeight(&cfg, &blk, 10*time.Second))
		blk, err = block.NewBuilder(block.RunnableActions{}).SetHeight(cfg.Genesis.DardanellesBlockHeight - 1).SignAndBuild(sk)
		r.NoError(err)
		r.Equal(blk.Height()+3, estimateTipHeight(&cfg, &blk, 20*time.Second))
	})
}
