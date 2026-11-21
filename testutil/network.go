// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	minRandomPort = 30000
	maxRandomPort = 50000
)

var (
	randomPortMu       sync.Mutex
	nextRandomPort     = rand.New(rand.NewSource(time.Now().UnixNano())).Intn(maxRandomPort-minRandomPort) + minRandomPort
	claimedRandomPorts = make(map[int]struct{})
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

// RandomPort returns an available port number between 30000 and 50000. Ports
// already handed out in this process are skipped even if their server has not
// started listening yet.
func RandomPort() int {
	randomPortMu.Lock()
	defer randomPortMu.Unlock()

	for range maxRandomPort - minRandomPort {
		port := nextRandomPort
		nextRandomPort++
		if nextRandomPort == maxRandomPort {
			nextRandomPort = minRandomPort
		}
		if _, claimed := claimedRandomPorts[port]; claimed {
			continue
		}
		if checkPortIsOpen(port) {
			continue
		}
		claimedRandomPorts[port] = struct{}{}
		return port
	}
	panic("no available test port between 30000 and 50000")
}
