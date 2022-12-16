// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"math/rand"
	"net"
	"strconv"
	"time"
)

func checkPortIsOpen(port int) bool {
	timeout := time.Millisecond * 10
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), timeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

// RandomPort returns a random port number between 30000 and 50000
func RandomPort() int {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var port int
	for port = r.Intn(2000) + 30000; port < 50000; port++ {
		if !checkPortIsOpen(port) {
			break
		}
	}
	return port
}
