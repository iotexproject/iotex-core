// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	pb "github.com/iotexproject/iotex-core/simulator/proto/simulator"
	"github.com/iotexproject/iotex-core/txpool"
)

const (
	port           = ":50051"
	rolldposConfig = "./config_local_rolldpos_sim.yaml"
	dummyMsgType   = 1999
)

// server is used to implement message.SimulatorServer.
type server struct {
	nodes []consensus.Sim // slice of Consensus objects
}

// Ping implements simulator.SimulatorServer
func (s *server) Init(in *pb.InitRequest, stream pb.Simulator_InitServer) error {
	fmt.Println()

	flag.Parse()

	var addrs []string // all delegate addresses
	for i := 0; i < int(in.NPlayers); i++ {
		addrs = append(addrs, "127.0.0.1:32"+strconv.Itoa(i))
	}

	for i := 0; i < int(in.NPlayers); i++ {
		cfg, err := config.LoadConfigWithPathWithoutValidation(rolldposConfig)
		if err != nil {
			logger.Error().Msg("Error loading config file")
		}

		//s.nodes = make([]consensus.Sim, in.NPlayers) // allocate all the necessary space now because otherwise nodes will get copied and create pointer issues

		// handle node address, delegate addresses, etc.
		cfg.Delegate.Addrs = addrs
		cfg.Network.Addr = addrs[i]

		// create public/private key pair and address
		chainID := make([]byte, 4)
		binary.LittleEndian.PutUint32(chainID, uint32(i))

		addr, err := iotxaddress.NewAddress(true, chainID)

		cfg.Chain.RawMinerAddr.PublicKey = hex.EncodeToString(addr.PublicKey)
		cfg.Chain.RawMinerAddr.PrivateKey = hex.EncodeToString(addr.PrivateKey)
		cfg.Chain.RawMinerAddr.RawAddress = addr.RawAddress

		// set chain database path
		cfg.Chain.ChainDBPath = "./chain" + strconv.Itoa(i) + ".db"

		bc := blockchain.CreateBlockchain(cfg, blockchain.Gen, nil)
		tp := txpool.NewTxPool(bc)

		overlay := network.NewOverlay(&cfg.Network)
		dlg := delegate.NewConfigBasedPool(&cfg.Delegate)
		bs := blocksync.NewBlockSyncer(cfg, bc, tp, overlay, dlg)
		bs.Start()

		node := consensus.NewSim(cfg, bc, tp, bs, dlg)

		s.nodes = append(s.nodes, node)

		done := make(chan bool)
		node.SetDoneStream(done)

		node.Start()

		fmt.Printf("Node %d initialized and consensus engine started\n", i)
		time.Sleep(5 * time.Millisecond)
		<-done

		fmt.Printf("Node %d initialization ended\n", i)

		//s.nodes = append(s.nodes, node)
	}

	fmt.Printf("Simulator initialized with %d players\n", in.NPlayers)

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
		s.nodes[in.PlayerID].HandleViewChange(msg, done)
		time.Sleep(5 * time.Millisecond)
		<-done // wait until done
	}

	fmt.Println("closed message stream")
	return nil
}

func (s *server) Exit(context context.Context, in *pb.Empty) (*pb.Empty, error) {
	defer os.Exit(0)
	return &pb.Empty{}, nil
}

func main() {
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
