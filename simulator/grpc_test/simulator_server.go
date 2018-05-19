package main

import (
	"log"
	"net"

	pb "github.com/iotexproject/iotex-core-internal/simulator/grpc_test/simulator"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	port = ":50051"
)

// server is used to implement message.SimulatorServer.
type server struct{}

// Ping implements simulator.SimulatorServer
func (s *server) Ping(ctx context.Context, in *pb.Request) (*pb.Reply, error) {
	return &pb.Reply{Message: "Hello " + in.Name}, nil
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
