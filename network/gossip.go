// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package network

import (
	"context"
	"sync"
	"time"

	"encoding/hex"

	"github.com/iotexproject/iotex-core/dispatch/dispatcher"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network/proto"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/routine"
	"github.com/iotexproject/iotex-core/proto"
)

// Gossip relays messages in the IotxOverlay (at least once semantics)
type Gossip struct {
	Overlay     *IotxOverlay
	Dispatcher  dispatcher.Dispatcher
	MsgLogs     *sync.Map
	CleanerTask *routine.RecurringTask

	lifecycle lifecycle.Lifecycle
}

// NewGossip generates a Gossip instance
func NewGossip(o *IotxOverlay) *Gossip {
	g := &Gossip{
		Overlay: o,
		MsgLogs: &sync.Map{},
	}
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
func (g *Gossip) OnReceivingMsg(msg *network.BroadcastReq) error {
	checksumStr := hex.EncodeToString(msg.MsgChecksum)
	if _, loaded := g.MsgLogs.LoadOrStore(checksumStr, time.Now()); loaded {
		return nil
	}
	// Call dispatch to notify that a new message comes in
	if err := g.processMsg(msg.ChainId, msg.MsgType, msg.MsgBody); err != nil {
		return err
	}
	// If other nodes use a crazy TTL, truncate it to the local configured value
	if msg.Ttl > g.Overlay.Config.TTL {
		msg.Ttl = g.Overlay.Config.TTL
	}
	// Relay the message to the neighbors
	if msg.Ttl-1 <= 0 {
		logger.Debug().
			Uint32("msg-type", msg.MsgType).
			Str("msg-checksum", hex.EncodeToString(msg.MsgChecksum)).
			Msg("message used up all delivery hops")
		return nil
	}
	if err := g.relayMsg(msg.ChainId, msg.MsgType, msg.MsgBody, msg.MsgChecksum, msg.Ttl-1); err != nil {
		return nil
	}
	return nil
}

func (g *Gossip) processMsg(chainID uint32, msgType uint32, msgBody []byte) error {
	protoMsg, err := iproto.TypifyProtoMsg(msgType, msgBody)
	if err != nil {
		return err
	}
	if g.Dispatcher != nil {
		g.Dispatcher.HandleBroadcast(chainID, protoMsg, nil)
	}
	return nil
}

func (g *Gossip) relayMsg(chainID uint32, msgType uint32, msgBody []byte, msgChecksum []byte, ttl int32) error {
	// Send the message to all neighbors
	g.Overlay.PM.Peers.Range(func(_, value interface{}) bool {
		go func() {
			peer, ok := value.(*Peer)
			if !ok {
				logger.Error().Msg("value is not an instance of Peer")
				return
			}
			_, err := peer.BroadcastMsg(
				&network.BroadcastReq{
					ChainId:     chainID,
					MsgType:     msgType,
					MsgBody:     msgBody,
					MsgChecksum: msgChecksum,
					Ttl:         ttl,
				},
			)
			if err != nil {
				logger.Error().
					Err(err).
					Str("dst", peer.String()).
					Str("msg-type", string(msgType)).
					Str("msg-checksum", hex.EncodeToString(msgChecksum)).
					Int32("ttl", ttl).
					Msg("failed to broadcast a message")
			}
		}()
		return true
	})
	return nil
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
	var keys []string
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
