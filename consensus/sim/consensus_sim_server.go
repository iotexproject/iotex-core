// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package sim

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"os"
	"runtime/pprof"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	pb "github.com/iotexproject/iotex-core/consensus/sim/proto"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/proto"
)

const (
	port         = ":50051"
	dummyMsgType = 1999
)

// server is used to implement message.SimulatorServer.
type (
	server struct {
		nodes []Sim // slice of Consensus objects
	}
	byzVal struct {
		val blockchain.Validator
	}
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

// Validate for the byzantine node uses the actual block validator and returns the opposite
func (v *byzVal) Validate(blk *blockchain.Block, tipHeight uint64, tipHash hash.Hash32B, containCoinbase bool) error {
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
		cfg.Consensus.RollDPoS.AcceptProposalEndorseTTL = 1000 * time.Second
		cfg.Consensus.RollDPoS.AcceptCommitEndorseTTL = 1000 * time.Second

		// handle node address, delegate addresses, etc.
		cfg.Network.Host = "127.0.0.1"
		cfg.Network.Port = 10000
		cfg.Network.NumPeersLowerBound = 6
		cfg.Network.NumPeersUpperBound = 12

		// create public/private key pair and address
		pk, sk, err := crypto.EC283.NewKeyPair()
		if err != nil {
			logger.Error().Err(err).Msg("failed to create public/private key pair together.")
		}
		cfg.Chain.ProducerPubKey = keypair.EncodePublicKey(pk)
		cfg.Chain.ProducerPrivKey = keypair.EncodePrivateKey(sk)

		// set chain database path
		cfg.Chain.ChainDBPath = "./chain" + strconv.Itoa(i) + ".db"
		cfg.Chain.TrieDBPath = "./trie" + strconv.Itoa(i) + ".db"

		bc := blockchain.NewBlockchain(cfg, blockchain.DefaultStateFactoryOption(), blockchain.BoltDBDaoOption())
		if err := bc.Start(ctx); err != nil {
			logger.Panic().Err(err).Msg("error when starting blockchain")
		}

		if i >= int(in.NFS+in.NHonest) { // is byzantine node
			val := bc.Validator()
			byzVal := &byzVal{val: val}
			bc.SetValidator(byzVal)
		}

		overlay := network.NewOverlay(cfg.Network)
		ap, err := actpool.NewActPool(bc, cfg.ActPool)
		if err != nil {
			logger.Fatal().Err(err).Msg("Fail to create actpool")
		}

		var node Sim
		if i < int(in.NHonest) {
			node = NewSim(cfg, bc, ap, overlay)
		} else if i < int(in.NHonest+in.NFS) {
			s.nodes = append(s.nodes, nil)
			continue
		} else {
			node = NewSimByzantine(cfg, bc, ap, overlay)
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
		msg := CombineMsg(in.InternalMsgType, msgValue)
		switch msg.(type) {
		case *iproto.ProposePb:
			err = s.nodes[in.PlayerID].HandleBlockPropose(msg.(*iproto.ProposePb), done)
		case *iproto.EndorsePb:
			err = s.nodes[in.PlayerID].HandleEndorse(msg.(*iproto.EndorsePb), done)
		}
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
			logger.Fatal().Err(err).Msg("failed to create file")
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to start CPU profile")
		}
	}

	lis, err := net.Listen("tcp", port)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to listen")
	}
	s := grpc.NewServer()
	pb.RegisterSimulatorServer(s, &server{})
	// Register reflection service on gRPC server.
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		logger.Fatal().Err(err).Msg("failed to serve")
	}
}
