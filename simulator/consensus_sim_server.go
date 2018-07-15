// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"runtime/pprof"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/hash"
	pb "github.com/iotexproject/iotex-core/simulator/proto/simulator"
)

const (
	port         = ":50051"
	dummyMsgType = 1999
)

// server is used to implement message.SimulatorServer.
type (
	server struct {
		nodes []consensus.Sim // slice of Consensus objects
	}
	byzVal struct {
		val blockchain.Validator
	}
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

// Validate for the byzantine node uses the actual block validator and returns the opposite
func (v *byzVal) Validate(blk *blockchain.Block, tipHeight uint64, tipHash hash.Hash32B) error {
	//err := v.val.Validate(blk, tipHeight, tipHash)
	//if err != nil {
	//	return nil
	//}
	//return errors.New("")
	return nil
}

// Ping implements simulator.SimulatorServer
func (s *server) Init(in *pb.InitRequest, stream pb.Simulator_InitServer) error {
	nPlayers := in.NBF + in.NFS + in.NHonest
	ctx := context.Background()

	addrs := []string{
		"io1qyqsyqcy6nm58gjd2wr035wz5eyd5uq47zyqpng3gxe7nh",
		"io1qyqsyqcy6m6hkqkj3f4w4eflm2gzydmvc0mumm7kgax4l3",
		"io1qyqsyqcyyu9pfazcx0wglp35h2h4fm0hl8p8z2u35vkcwc",
		"io1qyqsyqcyg9pk8zg8xzkmv6g3630xggvacq9e77cwtd4rkc",
	}

	for i := 0; i < int(nPlayers); i++ {
		cfg := config.Default
		// s.nodes = make([]consensus.Sim, in.NPlayers)
		// allocate all the necessary space now because otherwise nodes will get copied and create pointer issues
		cfg.Consensus.Scheme = config.RollDPoSScheme
		cfg.Consensus.RollDPoS.DelegateInterval = time.Millisecond
		cfg.Consensus.RollDPoS.ProposerInterval = 0
		cfg.Consensus.RollDPoS.UnmatchedEventTTL = 1000 * time.Second
		cfg.Consensus.RollDPoS.RoundStartTTL = 1000 * time.Second
		cfg.Consensus.RollDPoS.AcceptProposeTTL = 1000 * time.Second
		cfg.Consensus.RollDPoS.AcceptPrevoteTTL = 1000 * time.Second
		cfg.Consensus.RollDPoS.AcceptVoteTTL = 1000 * time.Second
		cfg.Consensus.RollDPoS.ProposerCB = "PseudoRotatedProposer"
		cfg.Consensus.RollDPoS.ProposerCB = "PseudoStarNewEpoch"

		// handle node address, delegate addresses, etc.
		cfg.Delegate.Addrs = addrs
		cfg.Network.Addr = "127.0.0.1:10000"
		cfg.Network.NumPeersLowerBound = 6
		cfg.Network.NumPeersUpperBound = 12

		// create public/private key pair and address
		addr, err := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
		if err != nil {
			logger.Error().Err(err).Msg("failed to create public/private key pair together with the address derived.")
		}
		cfg.Chain.ProducerPrivKey = hex.EncodeToString(addr.PrivateKey)
		cfg.Chain.ProducerPubKey = hex.EncodeToString(addr.PublicKey)

		// set chain database path
		cfg.Chain.ChainDBPath = "./chain" + strconv.Itoa(i) + ".db"
		cfg.Chain.TrieDBPath = "./trie" + strconv.Itoa(i) + ".db"

		bc := blockchain.NewBlockchain(&cfg, blockchain.DefaultStateFactoryOption(), blockchain.BoltDBDaoOption())

		if i >= int(in.NFS+in.NHonest) { // is byzantine node
			val := bc.Validator()
			byzVal := &byzVal{val: val}
			bc.SetValidator(byzVal)
		}

		overlay := network.NewOverlay(&cfg.Network)
		ap, err := actpool.NewActPool(bc, cfg.ActPool)
		if err != nil {
			logger.Fatal().Err(err).Msg("Fail to create actpool")
		}
		dlg := delegate.NewConfigBasedPool(&cfg.Delegate)
		bs, _ := blocksync.NewBlockSyncer(&cfg, bc, ap, overlay)
		err = bs.Start(ctx)
		if err != nil {
			logger.Fatal().Err(err).Msg("Fail to start blocksync")
		}

		var node consensus.Sim
		if i < int(in.NHonest) {
			node = consensus.NewSim(&cfg, bc, bs, dlg)
		} else if i < int(in.NHonest+in.NFS) {
			s.nodes = append(s.nodes, nil)
			continue
		} else {
			node = consensus.NewSimByzantine(&cfg, bc, bs, dlg)
		}

		s.nodes = append(s.nodes, node)

		done := make(chan bool)
		node.SetDoneStream(done)

		err = node.Start(ctx)
		if err != nil {
			logger.Fatal().Err(err).Msg("Fail to start node")
		}

		fmt.Printf("Node %d initialized and consensus engine started\n", i)
		time.Sleep(2 * time.Millisecond)
		<-done

		fmt.Printf("Node %d initialization ended\n", i)

		//s.nodes = append(s.nodes, node)
	}

	for i := 0; i < int(in.NFS); i++ {
		s.nodes = append(s.nodes, nil)
	}

	fmt.Printf("Simulator initialized with %d players\n", nPlayers)

	return nil
}

// Ping implements simulator.SimulatorServer
func (s *server) Ping(in *pb.Request, stream pb.Simulator_PingServer) error {
	fmt.Println()

	fmt.Printf("Node %d pinged; opened message stream\n", in.PlayerID)
	msgValue, err := hex.DecodeString(in.Value)
	if err != nil {
		logger.Error().Msg("Could not decode message value into byte array")
	}

	done := make(chan bool)

	s.nodes[in.PlayerID].SetStream(&stream)

	s.nodes[in.PlayerID].SendUnsent()

	// message type of 1999 means that it's a dummy message to allow the engine to pass back proposed blocks
	if in.InternalMsgType != dummyMsgType {
		msg := consensus.CombineMsg(in.InternalMsgType, msgValue)
		err = s.nodes[in.PlayerID].HandleViewChange(msg, done)
		if err != nil {
			logger.Error().Err(err).Msg("failed to handle view change")
		}
		time.Sleep(2 * time.Millisecond)
		<-done // wait until done
	}

	fmt.Println("closed message stream")
	return nil
}

func (s *server) Exit(context context.Context, in *pb.Empty) (*pb.Empty, error) {
	defer os.Exit(0)
	defer pprof.StopCPUProfile()
	return &pb.Empty{}, nil
}

func main() {
	flag.Parse()

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			log.Fatal(err)
		}
	}

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterSimulatorServer(s, &server{})
	// Register reflection service on gRPC server.
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
