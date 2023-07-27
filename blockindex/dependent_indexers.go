// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"

	"github.com/iotexproject/iotex-core/blockchain/blockdao"
)

type (
	// DependentIndexers is a struct that represents a group of indexers with dependencies between them.
	DependentIndexers struct {
		*IndexerGroup
	}

	// graphNodeWrapper is a wrapper of blockdao.BlockIndexer that implements the gonum graph.Node interface
	graphNodeWrapper struct {
		indexer blockdao.BlockIndexer
		id      int64
	}

	// graphNodeWrapperBuilder is a builder of graphNodeWrapper
	graphNodeWrapperBuilder struct {
		wrapperMap map[blockdao.BlockIndexer]*graphNodeWrapper
		count      int64
	}
)

// NewDependentIndexers creates a new dependent indexers.
// the dependencies are specified as pairs of indexers, where the first indexer depends on the second indexer.
// it uses a topological sort to determine the order in which the indexers should be added to the group
func NewDependentIndexers(dependencies ...[2]blockdao.BlockIndexer) (*DependentIndexers, error) {
	// build the graph
	g := simple.NewDirectedGraph()
	builder := &graphNodeWrapperBuilder{
		wrapperMap: make(map[blockdao.BlockIndexer]*graphNodeWrapper),
	}
	for _, dependency := range dependencies {
		g.SetEdge(g.NewEdge(builder.Build(dependency[0]), builder.Build(dependency[1])))
	}
	// topological sort
	order, err := topo.SortStabilized(g, nil)
	if err != nil {
		return nil, err
	}
	// build the indexers
	var indexers []blockdao.BlockIndexer
	for _, node := range order {
		indexers = append(indexers, node.(*graphNodeWrapper).indexer)
	}
	return &DependentIndexers{IndexerGroup: NewIndexerGroup(indexers...)}, nil
}

// ID returns the id of the wrapper
func (w *graphNodeWrapper) ID() int64 {
	return w.id
}

// Build builds a graphNodeWrapper
func (b *graphNodeWrapperBuilder) Build(indexer blockdao.BlockIndexer) *graphNodeWrapper {
	// return if the wrapper is already built
	if wrapper, ok := b.wrapperMap[indexer]; ok {
		return wrapper
	}

	// build the wrapper
	id := b.count
	b.count++
	wrapper := &graphNodeWrapper{
		indexer: indexer,
		id:      id,
	}
	b.wrapperMap[indexer] = wrapper
	return wrapper
}
