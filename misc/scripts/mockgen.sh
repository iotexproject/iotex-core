#!/bin/bash

mkdir -p ./test/mock

mkdir -p ./test/mock/mock_dispatcher
mockgen -destination=./test/mock/mock_dispatcher/mock_dispatcher.go  \
        -source=./dispatcher/dispatcher.go \
        -package=mock_dispatcher \
        Dispatcher

mkdir -p ./test/mock/mock_blockchain
mockgen -destination=./test/mock/mock_blockchain/mock_blockchain.go  \
        -source=./blockchain/blockchain.go \
        -package=mock_blockchain \
        Blockchain

mkdir -p ./test/mock/mock_blockdao
mockgen -destination=./test/mock/mock_blockdao/mock_blockdao.go  \
        -source=./blockchain/blockdao/blockdao.go \
        -aux_files=github.com/iotexproject/iotex-core/blockchain/blockdao=./blockchain/filedao/filedao.go \
        -package=mock_blockdao \
        BlockDAO

mkdir -p ./test/mock/mock_trie
mockgen -destination=./test/mock/mock_trie/mock_trie.go  \
        -source=./db/trie/trie.go \
        -package=mock_trie \
        Trie

mkdir -p ./test/mock/mock_factory
mockgen -destination=./test/mock/mock_factory/mock_factory.go  \
        -source=./state/factory/factory.go \
        -package=mock_factory \
        Factory

mkdir -p ./test/mock/mock_consensus
mockgen -destination=./test/mock/mock_consensus/mock_consensus.go  \
        -source=./consensus/consensus.go \
        -package=mock_consensus \
        Consensus

mockgen -destination=./consensus/consensusfsm/mock_context_test.go  \
        -source=./consensus/consensusfsm/context.go \
	-self_package=github.com/iotexproject/iotex-core/consensus/consensusfsm \
	-aux_files=github.com/iotexproject/iotex-core/consensus/consensusfsm=./consensus/consensusfsm/consensus_ttl.go \
	-package=consensusfsm \
        Context

mkdir -p ./test/mock/mock_lifecycle
mockgen -destination=./test/mock/mock_lifecycle/mock_lifecycle.go \
        github.com/iotexproject/iotex-core/pkg/lifecycle StartStopper

mkdir -p ./test/mock/mock_actpool
mockgen -destination=./test/mock/mock_actpool/mock_actpool.go  \
        -source=./actpool/actpool.go \
        -self_package=github.com/iotexproject/iotex-core/actpool \
        -package=mock_actpool \
        ActPool

mkdir -p ./test/mock/mock_actioniterator
mockgen -destination=./test/mock/mock_actioniterator/mock_actioniterator.go  \
        -source=./actpool/actioniterator/actioniterator.go \
        -package=mock_actioniterator \
        ActionIterator

mockgen -destination=./action/protocol/mock_protocol_test.go  \
        -source=./action/protocol/protocol.go \
        -self_package=github.com/iotexproject/iotex-core/action/protocol \
        -package=protocol \
        Protocol

mkdir -p ./test/mock/mock_poll
mockgen -destination=./test/mock/mock_poll/mock_poll.go  \
        -source=./action/protocol/poll/protocol.go \
        -package=mock_poll \
        Poll

mockgen -destination=./db/mock_kvstore.go  \
        -source=./db/kvstore.go \
        -self_package=github.com/iotexproject/iotex-core/db \
        -package=db \
        KVStore

mkdir -p ./test/mock/mock_batch
mockgen -destination=./test/mock/mock_batch/mock_batch.go  \
        -source=./db/batch/batch.go \
        -package=mock_batch \
        CachedBatch

mkdir -p ./test/mock/mock_sealed_envelope_validator
mockgen -destination=./test/mock/mock_sealed_envelope_validator/mock_sealed_envelope_validator.go  \
        -source=./action/sealedenvelopevalidator.go \
        -package=mock_sealed_envelope_validator \
        SealedEnvelopeValidator

mkdir -p ./test/mock/mock_chainmanager
mockgen -destination=./test/mock/mock_chainmanager/mock_chainmanager.go  \
        -source=./action/protocol/managers.go \
        -package=mock_chainmanager \
        StateManager

mkdir -p ./test/mock/mock_blocksync
mockgen -destination=./test/mock/mock_blocksync/mock_blocksync.go  \
        -source=./blocksync/blocksync.go \
        -self_package=github.com/iotexproject/iotex-core/blocksync \
        -package=mock_blocksync \
        BlockSync

mkdir -p ./test/mock/mock_blockcreationsubscriber
mockgen -destination=./test/mock/mock_blockcreationsubscriber/mock_blockcreationsubscriber.go \
        -source=./blockchain/blockcreationsubscriber.go \
        -package=mock_blockcreationsubscriber   \
        BlockCreationSubscriber

mkdir -p ./test/mock/mock_ioctlclient
mockgen -destination=./test/mock/mock_ioctlclient/mock_ioctlclient.go  \
        -source=./ioctl/client.go \
        -package=mock_ioctlclient \
        Client

mkdir -p ./test/mock/mock_apitypes
mockgen -destination=./test/mock/mock_apiresponder/mock_apitypes.go  \
        -source=./api/types/types.go \
        -package=mock_apitypes \
        APITypes

mkdir -p ./test/mock/mock_apiserver
mockgen -destination=./test/mock/mock_apiserver/mock_apiserver.go  \
        -source=./api/apitestserver.go \
        -package=mock_apiserver \
        StreamBlocksServer

mkdir -p ./test/mock/mock_apicoreservice
mockgen -destination=./test/mock/mock_apicoreservice/mock_apicoreservice.go  \
        -source=./api/coreservice.go \
        -package=mock_apicoreservice \
        CoreService

mkdir -p ./test/mock/mock_blockindex
mockgen -destination=./test/mock/mock_blockindex/mock_blockindex.go  \
        -source=./blockindex/bloomfilterindexer.go \
        -package=mock_blockindex \
        BlockIndex
        
mkdir -p ./test/mock/mock_web3server
mockgen -destination=./test/mock/mock_web3server/mock_web3server.go  \
        -source=./api/web3server.go \
        -package=mock_web3server \
        Web3Handler
