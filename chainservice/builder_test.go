package chainservice

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mohae/deepcopy"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/filedao"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestEstimateTipHeight(t *testing.T) {
	r := require.New(t)
	cfg := deepcopy.Copy(config.Default).(config.Config)
	cfg.Genesis.DardanellesBlockHeight = 10000
	cfg.Genesis.WakeBlockHeight = 20000
	sk := identityset.PrivateKey(1)
	t.Run("after dardanelles", func(t *testing.T) {
		blk, err := block.NewBuilder(block.RunnableActions{}).SetHeight(cfg.Genesis.DardanellesBlockHeight + 1).SignAndBuild(sk)
		r.NoError(err)
		r.Equal(blk.Height()+2, estimateTipHeight(&cfg, &blk, 10*time.Second))
		blk, err = block.NewBuilder(block.RunnableActions{}).SetHeight(cfg.Genesis.WakeBlockHeight - 3).SignAndBuild(sk)
		r.NoError(err)
		r.Equal(blk.Height()+2, estimateTipHeight(&cfg, &blk, 10*time.Second))
		r.Equal(blk.Height()+3, estimateTipHeight(&cfg, &blk, 12500*time.Millisecond))
		r.Equal(blk.Height()+3, estimateTipHeight(&cfg, &blk, 14500*time.Millisecond))
	})
	t.Run("before dardanelles", func(t *testing.T) {
		blk, err := block.NewBuilder(block.RunnableActions{}).SetHeight(cfg.Genesis.DardanellesBlockHeight - 100).SignAndBuild(sk)
		r.NoError(err)
		r.Equal(blk.Height()+1, estimateTipHeight(&cfg, &blk, 10*time.Second))
		blk, err = block.NewBuilder(block.RunnableActions{}).SetHeight(cfg.Genesis.DardanellesBlockHeight - 1).SignAndBuild(sk)
		r.NoError(err)
		r.Equal(blk.Height()+4, estimateTipHeight(&cfg, &blk, 20*time.Second))
	})
	t.Run("after wake", func(t *testing.T) {
		blk, err := block.NewBuilder(block.RunnableActions{}).SetHeight(cfg.Genesis.WakeBlockHeight).SignAndBuild(sk)
		r.NoError(err)
		r.Equal(blk.Height()+1, estimateTipHeight(&cfg, &blk, 2500*time.Millisecond))
		r.Equal(blk.Height()+1, estimateTipHeight(&cfg, &blk, 4*time.Second))
		r.Equal(blk.Height()+2, estimateTipHeight(&cfg, &blk, 5*time.Second))
	})
}

func TestBlockDistance(t *testing.T) {
	r := require.New(t)
	cfg := deepcopy.Copy(config.Default).(config.Config)
	cfg.Genesis.DardanellesBlockHeight = 10
	cfg.Genesis.WakeBlockHeight = 20
	t.Run("before dardanelles", func(t *testing.T) {
		r.Equal(3*cfg.Genesis.BlockInterval, blockDistance(&cfg, 5, 8, 0))
		r.Equal(10*cfg.Genesis.BlockInterval, blockDistance(&cfg, 5, 15, 0))
		r.Equal(20*cfg.Genesis.BlockInterval, blockDistance(&cfg, 5, 25, 0))
	})
	t.Run("after dardanelles before wake ", func(t *testing.T) {
		r.Equal(3*cfg.Genesis.BlockInterval, blockDistance(&cfg, 5, 8, 10))
		r.Equal(4*cfg.Genesis.BlockInterval+6*cfg.DardanellesUpgrade.BlockInterval, blockDistance(&cfg, 5, 15, 10))
		r.Equal(4*cfg.Genesis.BlockInterval+16*cfg.DardanellesUpgrade.BlockInterval, blockDistance(&cfg, 5, 25, 10))
	})
	t.Run("after wake", func(t *testing.T) {
		r.Equal(3*cfg.Genesis.BlockInterval, blockDistance(&cfg, 5, 8, 20))
		r.Equal(4*cfg.Genesis.BlockInterval+6*cfg.DardanellesUpgrade.BlockInterval, blockDistance(&cfg, 5, 15, 20))
		r.Equal(4*cfg.Genesis.BlockInterval+10*cfg.DardanellesUpgrade.BlockInterval+6*cfg.WakeUpgrade.BlockInterval, blockDistance(&cfg, 5, 25, 20))
	})
}

func TestBlockDistanceAt(t *testing.T) {
	r := require.New(t)
	heights := []uint64{0, 10, 20}
	intervals := []time.Duration{10 * time.Second, 5 * time.Second, 3 * time.Second}
	tests := []struct {
		name  string
		start uint64
		end   uint64
		want  time.Duration
	}{
		{name: "same start end",
			start: 100, end: 100,
			want: 0},
		{name: "within first interval",
			start: 0, end: 5,
			want: 50 * time.Second},
		{name: "cross first threshold",
			start: 5, end: 15,
			want: 70 * time.Second},
		{name: "boundary case",
			start: 0, end: 10,
			want: 95 * time.Second},
		{name: "cross first and second threshold",
			start: 5, end: 25,
			want: 108 * time.Second},
		{name: "start > end",
			start: 25, end: 5,
			want: 108 * time.Second},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blockDistanceAt(tt.start, tt.end, heights, intervals)
			r.Equal(tt.want, got, "test %d: %s", i, tt.name)
		})
	}
}

// TestBuildBlockDAOPropagatesStoreError guards against the block store
// construction error being swallowed. The "else" branch of buildBlockDAO used
// to redeclare err with ":=", shadowing the one the check after the block
// reads, so a chain DB that could not be opened produced no error and left
// cs.blockdao nil -- NewBlockDAOWithIndexersAndCache returns nil for a nil
// store. The node then continued building over that nil.
//
// The forTest branch was unaffected, which is why no existing test caught it.
func TestBuildBlockDAOPropagatesStoreError(t *testing.T) {
	r := require.New(t)

	// A directory where a bolt file is expected, so the store cannot be opened.
	badPath := filepath.Join(t.TempDir(), "chain.db")
	r.NoError(os.Mkdir(badPath, 0o755))

	cfg := deepcopy.Copy(config.Default).(config.Config)
	cfg.Chain.ChainDBPath = badPath
	cfg.Chain.BlobStoreDBPath = ""
	cfg.Chain.PatchReceiptIndexPath = ""
	cfg.Chain.PatchTransactionLogPath = ""

	// Without this the test could pass while proving nothing.
	dbCfg := cfg.DB
	dbCfg.DbPath = badPath
	_, ferr := filedao.NewFileDAO(dbCfg, block.NewDeserializer(cfg.Chain.EVMNetworkID))
	r.Error(ferr, "precondition: NewFileDAO must fail for this config")

	builder := NewBuilder(cfg)
	r.Error(builder.buildBlockDAO(false), "block store failure must be propagated, not swallowed")
}
