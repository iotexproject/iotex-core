// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"encoding/hex"
	"sync"
	"time"

	"golang.org/x/crypto/blake2b"

	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/common/routine"
	"github.com/iotexproject/iotex-core/common/service"
	pb "github.com/iotexproject/iotex-core/network/proto"
	pb1 "github.com/iotexproject/iotex-core/proto"
)

// Gossip relays messages in the overlay (at least once semantics)
type Gossip struct {
	service.CompositeService
	Overlay     *Overlay
	Dispatcher  cm.Dispatcher
	MsgLogs     sync.Map
	CleanerTask *routine.RecurringTask
}

// NewGossip generates a Gossip instance
func NewGossip(o *Overlay) *Gossip {
	g := &Gossip{Overlay: o}
	cleaner := NewMsgLogsCleaner(g)
	cleanerTask :=
		routine.NewRecurringTask(cleaner, o.Config.MsgLogsCleaningInterval)
	g.CleanerTask = cleanerTask
	g.AddService(cleanerTask)
	return g
}

// AttachDispatcher attaches to a Dispatcher instance
func (g *Gossip) AttachDispatcher(dispatcher cm.Dispatcher) {
	g.Dispatcher = dispatcher
}

// OnReceivingMsg listens to and handles the incoming broadcast message
func (g *Gossip) OnReceivingMsg(msg *pb.BroadcastReq) error {
	checksum := g.getBroadcastMsgChecksum(msg.MsgBody)
	_, ok := g.MsgLogs.Load(checksum)
	if ok {
		return nil
	}
	// Call dispatch to notify that a new message comes in
	err := g.processMsg(msg.MsgType, msg.MsgBody)
	if err != nil {
		return err
	}
	// Relay the message to the neighbors
	err = g.relayMsg(msg.MsgType, msg.MsgBody, checksum)
	if err != nil {
		return nil
	}
	return nil
}

func (g *Gossip) processMsg(msgType uint32, msgBody []byte) error {
	protoMsg, err := pb1.TypifyProtoMsg(msgType, msgBody)
	if err != nil {
		return err
	}
	if g.Dispatcher != nil {
		g.Dispatcher.HandleBroadcast(protoMsg, nil)
	}
	return nil
}

func (g *Gossip) relayMsg(msgType uint32, msgBody []byte, checksum string) error {
	// Record the message
	g.storeBroadcastMsgChecksum(checksum)
	// Send the message to all neighbors
	g.Overlay.PM.Peers.Range(func(_, value interface{}) bool {
		go func() {
			value.(*Peer).BroadcastMsg(&pb.BroadcastReq{MsgType: msgType, MsgBody: msgBody})
		}()
		return true
	})
	return nil
}

func (g *Gossip) getBroadcastMsgChecksum(msgBody []byte) string {
	b := blake2b.Sum256(msgBody)
	return hex.EncodeToString(b[:])
}

func (g *Gossip) storeBroadcastMsgChecksum(checksum string) {
	g.MsgLogs.Store(checksum, time.Now())
}

// MsgLogsCleaner periodically refreshes the recent received message log
type MsgLogsCleaner struct {
	G *Gossip
}

// NewMsgLogsCleaner generates a MsgLogsCleaner instance
func NewMsgLogsCleaner(g *Gossip) *MsgLogsCleaner {
	c := &MsgLogsCleaner{G: g}
	return c
}

// Do log cleaning
func (c *MsgLogsCleaner) Do() {
	keys := []string{}
	c.G.MsgLogs.Range(func(key, value interface{}) bool {
		if time.Now().Sub(value.(time.Time)) > c.G.Overlay.Config.MsgLogRetention {
			keys = append(keys, key.(string))
		}
		return true
	})
	for _, key := range keys {
		c.G.MsgLogs.Delete(key)
	}
}
