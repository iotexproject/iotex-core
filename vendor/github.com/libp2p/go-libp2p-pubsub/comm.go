package pubsub

import (
	"bufio"
	"context"
	"io"

	pb "github.com/libp2p/go-libp2p-pubsub/pb"

	ggio "github.com/gogo/protobuf/io"
	proto "github.com/gogo/protobuf/proto"
	inet "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	ms "github.com/multiformats/go-multistream"
)

// get the initial RPC containing all of our subscriptions to send to new peers
func (p *PubSub) getHelloPacket() *RPC {
	var rpc RPC
	for t := range p.myTopics {
		as := &pb.RPC_SubOpts{
			Topicid:   proto.String(t),
			Subscribe: proto.Bool(true),
		}
		rpc.Subscriptions = append(rpc.Subscriptions, as)
	}
	return &rpc
}

func (p *PubSub) handleNewStream(s inet.Stream) {
	r := ggio.NewDelimitedReader(s, 1<<20)
	for {
		rpc := new(RPC)
		err := r.ReadMsg(&rpc.RPC)
		if err != nil {
			if err != io.EOF {
				s.Reset()
				log.Infof("error reading rpc from %s: %s", s.Conn().RemotePeer(), err)
			} else {
				// Just be nice. They probably won't read this
				// but it doesn't hurt to send it.
				s.Close()
			}
			return
		}

		rpc.from = s.Conn().RemotePeer()
		select {
		case p.incoming <- rpc:
		case <-p.ctx.Done():
			// Close is useless because the other side isn't reading.
			s.Reset()
			return
		}
	}
}

func (p *PubSub) handleNewPeer(ctx context.Context, pid peer.ID, outgoing <-chan *RPC) {
	s, err := p.host.NewStream(p.ctx, pid, p.rt.Protocols()...)
	if err != nil {
		log.Warning("opening new stream to peer: ", err, pid)

		var ch chan peer.ID
		if err == ms.ErrNotSupported {
			ch = p.newPeerError
		} else {
			ch = p.peerDead
		}

		select {
		case ch <- pid:
		case <-ctx.Done():
		}
		return
	}

	go p.handleSendingMessages(ctx, s, outgoing)
	go p.handlePeerEOF(ctx, s)
	select {
	case p.newPeerStream <- s:
	case <-ctx.Done():
	}
}

func (p *PubSub) handlePeerEOF(ctx context.Context, s inet.Stream) {
	r := ggio.NewDelimitedReader(s, 1<<20)
	rpc := new(RPC)
	for {
		err := r.ReadMsg(&rpc.RPC)
		if err != nil {
			select {
			case p.peerDead <- s.Conn().RemotePeer():
			case <-ctx.Done():
			}
			return
		}
		log.Warning("unexpected message from ", s.Conn().RemotePeer())
	}
}

func (p *PubSub) handleSendingMessages(ctx context.Context, s inet.Stream, outgoing <-chan *RPC) {
	bufw := bufio.NewWriter(s)
	wc := ggio.NewDelimitedWriter(bufw)

	writeMsg := func(msg proto.Message) error {
		err := wc.WriteMsg(msg)
		if err != nil {
			return err
		}

		return bufw.Flush()
	}

	defer inet.FullClose(s)
	for {
		select {
		case rpc, ok := <-outgoing:
			if !ok {
				return
			}

			err := writeMsg(&rpc.RPC)
			if err != nil {
				s.Reset()
				log.Infof("writing message to %s: %s", s.Conn().RemotePeer(), err)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func rpcWithSubs(subs ...*pb.RPC_SubOpts) *RPC {
	return &RPC{
		RPC: pb.RPC{
			Subscriptions: subs,
		},
	}
}

func rpcWithMessages(msgs ...*pb.Message) *RPC {
	return &RPC{RPC: pb.RPC{Publish: msgs}}
}

func rpcWithControl(msgs []*pb.Message,
	ihave []*pb.ControlIHave,
	iwant []*pb.ControlIWant,
	graft []*pb.ControlGraft,
	prune []*pb.ControlPrune) *RPC {
	return &RPC{
		RPC: pb.RPC{
			Publish: msgs,
			Control: &pb.ControlMessage{
				Ihave: ihave,
				Iwant: iwant,
				Graft: graft,
				Prune: prune,
			},
		},
	}
}

func copyRPC(rpc *RPC) *RPC {
	res := new(RPC)
	*res = *rpc
	if rpc.Control != nil {
		res.Control = new(pb.ControlMessage)
		*res.Control = *rpc.Control
	}
	return res
}
