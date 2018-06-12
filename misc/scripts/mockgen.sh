#!/bin/bash

rm -rf ./test/mock
mkdir -p ./test/mock

mkdir -p ./test/mock/mock_dispatcher
mockgen -destination=./test/mock/mock_dispatcher/mock_dispatcher.go  \
        -source=./dispatch/dispatcher/dispatcher.go \
        -package=mock_dispatcher \
        Dispatcher

mkdir -p ./test/mock/mock_rolldpos
mockgen -destination=./test/mock/mock_rolldpos/mock_rolldpos.go  \
        -source=./consensus/scheme/rolldpos/rolldpos.go \
        -package=mock_rolldpos \
        DNet

mkdir -p ./test/mock/mock_blockchain
mockgen -destination=./test/mock/mock_blockchain/mock_blockchain.go  \
        -source=./blockchain/blockchain.go \
        -imports =github.com/iotexproject/iotex-core/blockchain \
        -package=mock_blockchain \
        Blockchain

mkdir -p ./test/mock/mock_delegate
mockgen -destination=./test/mock/mock_delegate/mock_delegate.go  \
        -source=./delegate/delegate.go \
        -package=mock_delegate \
        Pool

mkdir -p ./test/mock/mock_txpool
mockgen -destination=./test/mock/mock_txpool/mock_txpool.go  \
        -source=./txpool/txpool.go \
        -imports =github.com/iotexproject/iotex-core/txpool \
        -package=mock_txpool \
        TxPool

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

mkdir -p ./test/mock/mock_statefactory
mockgen -destination=./test/mock/mock_statefactory/mock_statefactory.go  \
        -source=./statefactory/statefactory.go \
        -imports =github.com/iotexproject/iotex-core/statefactory \
        -package=mock_statefactory \
        StateFactory
