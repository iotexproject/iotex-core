// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/delegate"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/network"
	pb "github.com/iotexproject/iotex-core/simulator/proto/simulator"
	"github.com/iotexproject/iotex-core/txpool"
)

const (
	port        = ":50051"
	rdposConfig = "./config_local_rdpos_sim.yaml"
)

// server is used to implement message.SimulatorServer.
type server struct {
	nodes []consensus.ConsensusSim
}

// Ping implements simulator.SimulatorServer
func (s *server) Init(in *pb.InitRequest, stream pb.Simulator_InitServer) error {
	fmt.Println()

	flag.Parse()

	consensus.Init = true // mark that we are in initialization phase

	for i := 0; i < int(in.NPlayers); i++ {
		cfg, err := config.LoadConfigWithPathWithoutValidation(rdposConfig)
		if err != nil {
			glog.Error("Error loading config file")
		}

		// create public/private key pair and address
		chainID := make([]byte, 4)
		binary.LittleEndian.PutUint32(chainID, uint32(i))

		addr, err := iotxaddress.NewAddress(true, chainID)

		cfg.Chain.RawMinerAddr.PublicKey = hex.EncodeToString(addr.PublicKey)
		cfg.Chain.RawMinerAddr.PrivateKey = hex.EncodeToString(addr.PrivateKey)
		cfg.Chain.RawMinerAddr.RawAddress = addr.RawAddress

		// set chain database path
		cfg.Chain.ChainDBPath = "./chain" + strconv.Itoa(i) + ".db"

		bc := blockchain.CreateBlockchain(cfg, blockchain.Gen)
		tp := txpool.New(bc)

		overlay := network.NewOverlay(&cfg.Network)
		dlg := delegate.NewConfigBasedPool(&cfg.Delegate)
		bs := blocksync.NewBlockSyncer(cfg, bc, tp, overlay, dlg)

		node := consensus.NewConsensusSim(cfg, bc, tp, bs, dlg)
		fmt.Printf("%p\n", node)

		done := make(chan bool)
		node.SetDoneStream(done)
		node.SetInitStream(&stream)
		node.SetID(i)

		node.Start()

		fmt.Printf("Node %d initialized and consensus engine started\n", i)
		time.Sleep(time.Second)
		<-done

		fmt.Printf("Node %d initialization ended\n", i)

		s.nodes = append(s.nodes, node)
	}

	fmt.Printf("Simulator initialized with %d players\n", in.NPlayers)

	return nil
}

// Ping implements simulator.SimulatorServer
func (s *server) Ping(in *pb.Request, stream pb.Simulator_PingServer) error {
	fmt.Println()

	consensus.Init = false // mark that we are not in initialization phase any more

	fmt.Println("opened message stream")
	msgValue, err := hex.DecodeString(in.Value)
	if err != nil {
		glog.Error("Could not decode message value into byte array")
	}

	msg := consensus.CombineMsg(in.InternalMsgType, msgValue)

	done := make(chan bool)

	s.nodes[in.PlayerID].SetStream(&stream)
	s.nodes[in.PlayerID].CheckIfStreamNil()

	fmt.Println("Sent message for handling")
	s.nodes[in.PlayerID].HandleViewChange(msg, done)

	<-done
	fmt.Println("closed message stream")
	return nil
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
