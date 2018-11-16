#!/bin/bash

rm -rf ./test/mock
mkdir -p ./test/mock

mkdir -p ./test/mock/mock_dispatcher
mockgen -destination=./test/mock/mock_dispatcher/mock_dispatcher.go  \
        -source=./dispatcher/dispatcher.go \
        -package=mock_dispatcher \
        Dispatcher

mkdir -p ./test/mock/mock_blockchain
mockgen -destination=./test/mock/mock_blockchain/mock_blockchain.go  \
        -source=./blockchain/blockchain.go \
        -imports =github.com/iotexproject/iotex-core/blockchain \
        -package=mock_blockchain \
        Blockchain

mkdir -p ./test/mock/mock_blocksync
mockgen -destination=./test/mock/mock_blocksync/mock_blocksync.go  \
        -source=./blocksync/blocksync.go \
        -package=mock_blocksync \
        BlockSync

mkdir -p ./test/mock/mock_trie
mockgen -destination=./test/mock/mock_trie/mock_trie.go  \
        -source=./trie/trie.go \
        -package=mock_trie \
        Trie

mkdir -p ./test/mock/mock_factory
mockgen -destination=./test/mock/mock_factory/mock_factory.go  \
        -source=./state/factory/factory.go \
        -imports =github.com/iotexproject/iotex-core/state/factory \
        -package=mock_factory \
        Factory

mkdir -p ./test/mock/mock_factory
mockgen -destination=./test/mock/mock_factory/mock_workingset.go  \
        -source=./state/factory/workingset.go \
        -imports =github.com/iotexproject/iotex-core/state/factory \
        -package=mock_factory \
        WorkingSet

mkdir -p ./test/mock/mock_consensus
mockgen -destination=./test/mock/mock_consensus/mock_consensus.go  \
        -source=./consensus/consensus.go \
        -imports =github.com/iotexproject/iotex-core/consensus \
        -package=mock_consensus \
        Consensus

mkdir -p ./test/mock/mock_network
mockgen -destination=./test/mock/mock_network/mock_overlay.go  \
        -source=./network/overlay.go \
        -package=mock_network \
        Overlay

mkdir -p ./test/mock/mock_lifecycle
mockgen -destination=./test/mock/mock_lifecycle/mock_lifecycle.go \
        github.com/iotexproject/iotex-core/pkg/lifecycle StartStopper

mkdir -p ./test/mock/mock_actpool
mockgen -destination=./test/mock/mock_actpool/mock_actpool.go  \
        -source=./actpool/actpool.go \
        -package=mock_actpool \
        ActPool

mkdir -p ./test/mock/mock_explorer
mockgen -destination=./test/mock/mock_explorer/mock_explorer.go  \
        -source=./explorer/idl/explorer/explorer.go \
        -package=mock_explorer \
        Explorer
