// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"net"

<<<<<<< HEAD
	pb "github.com/iotexproject/iotex-core/simulator/proto/simulator"
=======
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	pb "github.com/iotexproject/iotex-core-internal/simulator/proto/simulator"
>>>>>>> Modify init message and modify ping message definition
)

const (
	port = ":50051"
)

// server is used to implement message.SimulatorServer.
type server struct {
	count int
}

func (s *server) Init(ctx context.Context, in *pb.InitRequest) (*pb.Empty, error) {
	fmt.Printf("initialized with %d players\n", in.NPlayers)
	return &pb.Empty{}, nil
}

// Ping implements simulator.SimulatorServer
func (s *server) Ping(in *pb.Request, stream pb.Simulator_PingServer) error {
	s.count++

	fmt.Println("count", s.count)
	stream.Send(&pb.Reply{MessageType: 3, Value: "block 66"})
	stream.Send(&pb.Reply{MessageType: 2, Value: "block 4"})
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
