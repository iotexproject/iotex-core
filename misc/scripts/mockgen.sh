#!/bin/bash

rm -rf ./test/mock
mkdir -p ./test/mock

mkdir -p ./test/mock/mock_dispatcher
mockgen -destination=./test/mock/mock_dispatcher/mock_dispatcher.go  \
        -source=./common/idispatcher.go \
        -package=mock_dispatcher \
        Dispatcher

mkdir -p ./test/mock/mock_rdpos
mockgen -destination=./test/mock/mock_rdpos/mock_rdpos.go  \
        -source=./consensus/scheme/rdpos/rdpos.go \
        -package=mock_rdpos \
        DNet

mkdir -p ./test/mock/mock_blockchain
mockgen -destination=./test/mock/mock_blockchain/mock_blockchain.go  \
        -source=./blockchain/blockchain.go \
        -imports =github.com/iotexproject/iotex-core-internal/blockchain \
        -package=mock_blockchain \
        Blockchain

mkdir -p ./test/mock/mock_delegate
mockgen -destination=./test/mock/mock_delegate/mock_delegate.go  \
        -source=./delegate/delegate.go \
        -package=mock_delegate \
        Delegate

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
