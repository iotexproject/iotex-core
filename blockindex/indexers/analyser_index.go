package indexers

import "github.com/iotexproject/iotex-core/blockchain/blockdao"

// TODO AnalyserIndexer or BlockIndexer
type AnalyserIndexer interface {
	blockdao.BlockIndexer
	Name() string
	Version() string
}

type DependentAnalyserIndexer interface {
	DependentIndexer() []string
}
