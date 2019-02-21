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

	"github.com/iotexproject/iotex-core/protogen/iotexapi"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

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
	// fmt.Println(servers)
	for _, bcServer := range servers {
		// fmt.Println(bcServer)
		// TODO - run for all servers in goroutines ?
		c, err := New(bcServer)
		if err != nil {
			errMsg := fmt.Errorf("could not connect to server %s", bcServer)
			fmt.Println(errMsg)
			continue
		}
		res, err := c.ServerMonitor(height)
		if err != nil {
			errMsg := fmt.Errorf("error in getting health status for %s, error %v", bcServer, err)
			fmt.Println(errMsg)
		}
		if res {
			successMsg := fmt.Sprintf("positive health status for server %s", bcServer)
			fmt.Println(successMsg)
		} else {
			failureMsg := fmt.Sprintf("negative health status for server %s", bcServer)
			fmt.Println(failureMsg)
		}
	}
}

// Client is the blockchain API client.
type Client struct {
	api iotexapi.APIServiceClient
}

// New creates a new Client.
func New(serverAddr string) (*Client, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	return &Client{
		api: iotexapi.NewAPIServiceClient(conn),
	}, nil
}

// ServerMonitor is to track server's health by checking it's current height
func (c *Client) ServerMonitor(height uint64) (bool, error) {
	req := &iotexapi.GetChainMetaRequest{}
	resp, err := c.api.GetChainMeta(context.Background(), req)
	if err != nil {
		return false, fmt.Errorf("\n error in GetChainMeta: %v", err)
	}
	fmt.Printf("Info: Current height is %v", resp.ChainMeta.Height)
	if resp.ChainMeta.Height < height {
		return false, nil
	}

	return true, nil
}
