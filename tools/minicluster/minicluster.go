// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// usage: make minicluster

package main

import (
	"math/big"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"

	"github.com/iotexproject/iotex-core/config"
)

const (
	_numNodes  = 4
	_numAdmins = 2
)

func newConfig(
	producerPriKey crypto.PrivateKey,
	networkPort,
	apiPort int,
	web3APIPort int,
	webSocketPort int,
	HTTPAdminPort int,
) config.Config {
	cfg := config.Default

	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.EnableAsyncIndexWrite = false

	cfg.System.HTTPAdminPort = HTTPAdminPort
	cfg.Network.Port = networkPort
	cfg.Network.BootstrapNodes = []string{"/ip4/127.0.0.1/tcp/4689/ipfs/12D3KooWJwW6pUpTkxPTMv84RPLPMQVEAjZ6fvJuX4oZrvW5DAGQ"}

	cfg.Chain.ID = 1
	cfg.Chain.ProducerPrivKey = producerPriKey.HexString()

	cfg.ActPool.MinGasPriceStr = big.NewInt(0).String()

	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Consensus.RollDPoS.FSM.UnmatchedEventInterval = 2400 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.AcceptBlockTTL = 1800 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.AcceptProposalEndorsementTTL = 1800 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.AcceptLockEndorsementTTL = 1800 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.CommitTTL = 600 * time.Millisecond
	cfg.Consensus.RollDPoS.FSM.EventChanSize = 100000
	cfg.Consensus.RollDPoS.ToleratedOvertime = 1200 * time.Millisecond
	cfg.Consensus.RollDPoS.Delay = 6 * time.Second

	cfg.API.GRPCPort = apiPort
	cfg.API.HTTPPort = web3APIPort
	cfg.API.WebSocketPort = webSocketPort

	cfg.Genesis.BlockInterval = 6 * time.Second
	cfg.Genesis.Blockchain.NumSubEpochs = 2
	cfg.Genesis.Blockchain.NumDelegates = _numNodes
	cfg.Genesis.Blockchain.TimeBasedRotation = true
	cfg.Genesis.Delegates = cfg.Genesis.Delegates[3 : _numNodes+3]
	cfg.Genesis.EnableGravityChainVoting = false
	cfg.Genesis.PollMode = "lifeLong"

	return cfg
}
