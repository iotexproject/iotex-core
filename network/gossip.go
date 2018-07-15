// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"context"
	"encoding/hex"
	"sync"
	"time"

	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/dispatch/dispatcher"
	"github.com/iotexproject/iotex-core/logger"
	pb "github.com/iotexproject/iotex-core/network/proto"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/routine"
	pb1 "github.com/iotexproject/iotex-core/proto"
)

// Gossip relays messages in the IotxOverlay (at least once semantics)
type Gossip struct {
	Overlay     *IotxOverlay
	Dispatcher  dispatcher.Dispatcher
	MsgLogs     sync.Map
	CleanerTask *routine.RecurringTask

	lifecycle lifecycle.Lifecycle
}

// NewGossip generates a Gossip instance
func NewGossip(o *IotxOverlay) *Gossip {
	g := &Gossip{Overlay: o}
	cleaner := NewMsgLogsCleaner(g)
	g.CleanerTask = routine.NewRecurringTask(cleaner.Clean, o.Config.MsgLogsCleaningInterval)
	g.lifecycle.Add(g.CleanerTask)
	return g
}

// Start starts Gossip.
func (g *Gossip) Start(ctx context.Context) error { return g.lifecycle.OnStart(ctx) }

// Stop stops Gossip.
func (g *Gossip) Stop(ctx context.Context) error { return g.lifecycle.OnStop(ctx) }

// AttachDispatcher attaches to a Dispatcher instance
func (g *Gossip) AttachDispatcher(dispatcher dispatcher.Dispatcher) {
	g.Dispatcher = dispatcher
}

// OnReceivingMsg listens to and handles the incoming broadcast message
func (g *Gossip) OnReceivingMsg(msg *pb.BroadcastReq) error {
	checksum := g.getBroadcastMsgChecksum(msg.MsgBody)
	_, ok := g.MsgLogs.Load(checksum)
	if ok {
		return nil
	}
	// Record the message
	g.storeBroadcastMsgChecksum(checksum)
	// Call dispatch to notify that a new message comes in
	err := g.processMsg(msg.MsgType, msg.MsgBody)
	if err != nil {
		return err
	}
	// Relay the message to the neighbors
	if msg.Ttl == 0 && msg.Ttl-1 == 0 {
		logger.Debug().
			Str("name", g.Overlay.RPC.String()).
			Uint32("msg", msg.MsgType).
			Msg("message used up all delivery hops")
		return nil
	}
	err = g.relayMsg(msg.MsgType, msg.MsgBody, msg.Ttl-1)
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

func (g *Gossip) relayMsg(msgType uint32, msgBody []byte, ttl uint32) error {
	// Send the message to all neighbors
	g.Overlay.PM.Peers.Range(func(_, value interface{}) bool {
		go func() {
			_, err := value.(*Peer).BroadcastMsg(&pb.BroadcastReq{MsgType: msgType, MsgBody: msgBody, Ttl: ttl})
			if err != nil {
				logger.Error().Err(err).Str("MsgType", string(msgType)).Msg("failed to broadcast msg")
			}
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

// Clean cleans logs
func (c *MsgLogsCleaner) Clean() {
	keys := []string{}
	c.G.MsgLogs.Range(func(key, value interface{}) bool {
		if time.Since(value.(time.Time)) > c.G.Overlay.Config.MsgLogRetention {
			keys = append(keys, key.(string))
		}
		return true
	})
	for _, key := range keys {
		c.G.MsgLogs.Delete(key)
	}
}
