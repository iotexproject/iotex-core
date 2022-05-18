// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

// Node is the root struct to embed the network identifier
type Node struct {
	NetworkType string
	Addr        string
}

// Network returns the transportation layer type (tcp by default)
func (n *Node) Network() string {
	if n.NetworkType == "" {
		return "tcp"
	}
	return n.NetworkType
}

// String returns the network address (127.0.0.1:0 by default)
func (n *Node) String() string {
	if n.Addr == "" {
		return "127.0.0.1:0"
	}
	return n.Addr
}

// NewTCPNode creates an instance of Peer with tcp transportation
func NewTCPNode(addr string) *Node {
	return NewNode("tcp", addr)
}

// NewNode creates an instance of Peer
func NewNode(n string, addr string) *Node {
	return &Node{NetworkType: n, Addr: addr}
}
