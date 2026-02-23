package actpool

import (
	"math/big"
	"time"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
)

var (
	// DefaultConfig is the default config for actpool
	DefaultConfig = Config{
		MaxNumActsPerPool:  32000,
		MaxGasLimitPerPool: 320000000,
		MaxNumActsPerAcct:  2000,
		WorkerBufferSize:   2000,
		ActionExpiry:       10 * time.Minute,
		MinGasPriceStr:     big.NewInt(unit.Qev).String(),
		BlackList: []string{
			"io10epxv6w4he9pgx0qtagm7lc78aw9jsm7s8t0tw",
			"io10jr2hh4xcm3s3yhq98zudxh08e9uume6e0ekee",
			"io126e8lcld9ucl6nxm55s6hxmq9velxw9u9t8ps5",
			"io13pgqh3dhd63sen5clvt2cvqflshf0swxzq4utt",
			"io1470pxq9qusz36pk4ljny446pwmkg9pvuzeshlj",
			"io147r62u9rf2szarfnn6pxse53vmkahlgk9s69ru",
			"io14y3zwpnpd5uv8clanfr73xnkxz2mk2qgjcgngu",
			"io153n6d37236qjaxtmlegvunnejx4dqz5gdeq889",
			"io15526y7w3n24dhjsk958ge23aprs9urrte3jlr3",
			"io16zkt2tg3rv9wnunjdsffywdhw7ceyhpph7v3r5",
			"io17n3utl90flcnn9wlskx8c7lk8rmcrf5th86273",
			"io19rw6y85rh4d78az0s7w5dknp6j06cst5wtwnt3",
			"io1aj8u2a0tr2mjumcxy4zrnecszr8mh6wkrmjrxq",
			"io1d97phgd9uvrcw9yylucx5drwuwl8s7eef8mh0p",
			"io1e3uf73n3vfyev3hhhkwx6phnatduqtp7l2hvjr",
			"io1fp748ue6tsssn3z9q5cxcpyewjvcptkrtryju3",
			"io1fqv56nlmr2fnuex66j09k0qn5962yeezkp688n",
			"io1frcfdlrtrk0kk354sl703e2v7qwd3rclajhg5r",
			"io1fseuvqpg8gchcnc4sz3e2z73h8q837ult8yjv6",
			"io1fsrrc02mldtwhjjncngvf58yxw2emmgsz9gk0p",
			"io1ft54ueh2qn0nfep6xf92nm60n5sqlj0d9uzksg",
			"io1ftl20mky24k43yp06zcjx653ktas9xmvzvtasz",
			"io1g0k4e2km87l0vyx6mz4wvg23nvstxn0x6vddn7",
			"io1jyclx2jhdpe0ljn38683ga26hnpucx03v9elmn",
			"io1llaf2w7kpek7zv2gqe6u9mkvfmd20h3ael6c2u",
			"io1rvfvt9hdyu6fe3f7z5cj8a4vqhn8yk5eync2jp",
			"io1rxzrf2q3letflu36p2w097ekkz4da4wuk7wnxw",
			"io1va6umgyewzjatq8nrznyct9f2yp49rkpxtx3jj",
			"io1zh88jlem8vvzp9z6t73rs4qd72jnzpm8pv8ndu",
		},
		MaxNumBlobsPerAcct: 16,
		Store: &StoreConfig{
			Datadir: "/var/data/actpool.cache",
		},
	}
)

// Config is the actpool config
type Config struct {
	// MaxNumActsPerPool indicates maximum number of actions the whole actpool can hold
	MaxNumActsPerPool uint64 `yaml:"maxNumActsPerPool"`
	// MaxGasLimitPerPool indicates maximum gas limit the whole actpool can hold
	MaxGasLimitPerPool uint64 `yaml:"maxGasLimitPerPool"`
	// MaxNumActsPerAcct indicates maximum number of actions an account queue can hold
	MaxNumActsPerAcct uint64 `yaml:"maxNumActsPerAcct"`
	// WorkerBufferSize indicates the buffer size for each worker's job queue
	WorkerBufferSize uint64 `yaml:"bufferPerAcct"`
	// ActionExpiry defines how long an action will be kept in action pool.
	ActionExpiry time.Duration `yaml:"actionExpiry"`
	// MinGasPriceStr defines the minimal gas price the delegate will accept for an action
	MinGasPriceStr string `yaml:"minGasPrice"`
	// BlackList lists the account address that are banned from initiating actions
	BlackList []string `yaml:"blackList"`
	// Store defines the config for persistent cache
	Store *StoreConfig `yaml:"store"`
	// MaxNumBlobsPerAcct defines the maximum number of blob txs an account can have
	MaxNumBlobsPerAcct uint64 `yaml:"maxNumBlobsPerAcct"`
}

// MinGasPrice returns the minimal gas price threshold
func (ap Config) MinGasPrice() *big.Int {
	mgp, ok := new(big.Int).SetString(ap.MinGasPriceStr, 10)
	if !ok {
		log.S().Panicf("Error when parsing minimal gas price string: %s", ap.MinGasPriceStr)
	}
	return mgp
}

// StoreConfig is the configuration for the blob store
type StoreConfig struct {
	Datadir string `yaml:"datadir"` // Data directory containing the currently executable blobs
}
