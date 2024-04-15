package actsync

import (
	"context"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	retryTimes     = 3
	unicaseTimeout = 5 * time.Second
)

type (
	// Neighbors acquires p2p neighbors in the network
	Neighbors func() ([]peer.AddrInfo, error)
	// UniCastOutbound sends a unicase message to the peer
	UniCastOutbound func(context.Context, peer.AddrInfo, proto.Message) error
	// ReceiveCallback is the callback function when receiving an action
	ReceiveCallback func(*action.SealedEnvelope)
	// GetAction gets an action by hash
	GetAction func(hash hash.Hash256) *action.SealedEnvelope

	// ActionSync implements the action syncer
	ActionSync struct {
		actions *sync.Map

		p2pNeighbor     Neighbors
		unicastOutbound UniCastOutbound
		receiveCallback ReceiveCallback
		getAction       GetAction
	}

	actionMsg struct {
		response chan *action.SealedEnvelope
	}
)

func NewActionSync(p2pNeighbor Neighbors, unicastOutbound UniCastOutbound, receiveCallback ReceiveCallback, getAction GetAction) *ActionSync {
	return &ActionSync{
		actions:         &sync.Map{},
		p2pNeighbor:     p2pNeighbor,
		unicastOutbound: unicastOutbound,
		receiveCallback: receiveCallback,
		getAction:       getAction,
	}
}

func (as *ActionSync) RequestAction(ctx context.Context, hash hash.Hash256) error {
	// check if the action is already requested
	act, ok := as.actions.LoadOrStore(hash, &actionMsg{
		response: make(chan *action.SealedEnvelope),
	})
	if ok {
		log.L().Debug("Action already requested", log.Hex("hash", hash[:]))
		return nil
	}
	requestFromNeighbors := func() error {
		neighbors, err := as.p2pNeighbor()
		if err != nil {
			return err
		}
		for i := range neighbors {
			uniCtx, _ := context.WithTimeout(ctx, unicaseTimeout)
			if err := as.unicastOutbound(uniCtx, neighbors[i], &iotexrpc.ActionSync{Hashes: [][]byte{hash[:]}}); err != nil {
				log.L().Warn("Failed to send action request", zap.Error(err), zap.String("peer", neighbors[i].String()))
				continue
			}
			select {
			case <-ctx.Done():
				// the whole request is canceled
				return ctx.Err()
			case <-uniCtx.Done():
				// unicast timeout, request from other neighbors
				continue
			case response := <-act.(*actionMsg).response:
				if response == nil {
					// no response, request from other neighbors
					continue
				}
				// action received
				as.receiveCallback(response)
				as.actions.Delete(hash)
				return nil
			}
		}
		return errors.Errorf("failed to request action %x from neighbors", common.Hash(hash[:]).Hex())
	}
	var err error
	for i := 0; i < retryTimes; i++ {
		if err = requestFromNeighbors(); err != nil {
			log.L().Warn("Failed to request action from neighbors", zap.Error(err))
			continue
		}
		return nil
	}
	return err
}

func (as *ActionSync) ResponceActionRequest(ctx context.Context, hash hash.Hash256, peer peer.AddrInfo) error {
	actions := &iotextypes.ActionReply{
		Hashes:  [][]byte{hash[:]},
		Actions: []*iotextypes.Action{nil},
	}
	if act := as.getAction(hash); act != nil {
		actions.Actions[0] = act.Proto()
	}
	return as.unicastOutbound(ctx, peer, actions)
}

func (as *ActionSync) ReceiveAction(ctx context.Context, hash hash.Hash256, selp *action.SealedEnvelope) {
	act, ok := as.actions.Load(hash)
	if !ok {
		log.L().Warn("action not requested or already received", log.Hex("hash", hash[:]))
		return
	}
	select {
	case act.(*actionMsg).response <- selp:
	default:
	}
}
