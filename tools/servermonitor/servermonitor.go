// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

// This is a testing tool to check for health of running blockchain server
// To use, run "make build" and " ./bin/servermonitor -endpoints=127.0.0.1:14014,127.0.0.2:14014 -height=1234"

//TODO - Modify makefile to add support for it ?

package main

import (
	"fmt"
	"strings"

	"flag"

	pb "github.com/iotexproject/iotex-core/protogen/iotexapi"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var client pb.APIServiceClient

func main() {
	// target addresses for jrpc connection.
	var addr string
	// expected height of blockchain
	var height uint64

	flag.StringVar(&addr, "endpoints", "", "server addresses in format ip1:port1,ip2:port2 ")
	flag.Uint64Var(&height, "height", 0, "expected height of blockchain")
	flag.Parse()

	// parse cli addresses to collect different servers
	splitFn := func(c rune) bool {
		return c == ','
	}
	servers := strings.FieldsFunc(addr, splitFn)

	// check health for all given servers
	for _, bcServer := range servers {
		// TODO - run for all servers in goroutines ?
		// log.Println(bcServer)
		conn, err := grpc.Dial(bcServer, grpc.WithInsecure())

		if err != nil {
			errMsg := fmt.Errorf("cannot dial server %s : %v", bcServer, err)
			fmt.Println(errMsg)
			continue
		}

		client = pb.NewAPIServiceClient(conn)

		req := &pb.GetChainMetaRequest{}
		resp, err := client.GetChainMeta(context.Background(), req)
		if err != nil {
			errMsg := fmt.Errorf("error in getting health status for %s, error %v", bcServer, err)
			fmt.Println(errMsg)
			continue
		}
		// check for returned height
		if resp.ChainMeta.Height > height {
			successMsg := fmt.Sprintf("positive health status for server %s", bcServer)
			fmt.Println(successMsg)
		} else {
			failureMsg := fmt.Sprintf("negative health status for server %s", bcServer)
			fmt.Println(failureMsg)
		}
	}
}
