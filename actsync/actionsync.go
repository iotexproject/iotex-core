package actsync

import (
	"context"
	"sync"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotexrpc"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/pkg/fastrand"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

const (
	unicaseTimeout = time.Second
	batchPeerSize  = 2
)

type (
	// Neighbors acquires p2p neighbors in the network
	Neighbors func() ([]peer.AddrInfo, error)
	// UniCastOutbound sends a unicase message to the peer
	UniCastOutbound func(context.Context, peer.AddrInfo, proto.Message) error

	// ActionSync implements the action syncer
	ActionSync struct {
		lifecycle.Readiness
		actions  sync.Map
		syncChan chan hash.Hash256
		wg       sync.WaitGroup
		helper   *Helper
		cfg      Config
		quit     chan struct{}
	}

	actionMsg struct {
		lastTime time.Time
	}
)

var (
	channelFullnessMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_actionsync_fullness",
			Help: "ActionSync fullness statistics",
		},
		[]string{"message_type"},
	)
	counterMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_actionsync_counter",
			Help: "ActionSync counter statistics",
		},
		[]string{"message_type"},
	)
)

func init() {
	prometheus.MustRegister(channelFullnessMtc, counterMtc)
}

// NewActionSync creates a new action syncer
func NewActionSync(cfg Config, helper *Helper) *ActionSync {
	return &ActionSync{
		syncChan: make(chan hash.Hash256, cfg.Size),
		helper:   helper,
		cfg:      cfg,
		quit:     make(chan struct{}),
	}
}

// Start starts the action syncer
func (as *ActionSync) Start(ctx context.Context) error {
	log.L().Info("starting action sync")
	as.wg.Add(1)
	go as.sync()

	as.wg.Add(1)
	go as.triggerSync()

	return as.TurnOn()
}

// Stop stops the action syncer
func (as *ActionSync) Stop(ctx context.Context) error {
	log.L().Info("stopping action sync")
	if err := as.TurnOff(); err != nil {
		return err
	}
	close(as.quit)
	as.wg.Wait()
	return nil
}

// RequestAction requests an action by hash
func (as *ActionSync) RequestAction(_ context.Context, hash hash.Hash256) {
	if !as.IsReady() {
		return
	}
	// check if the action is already requested
	_, ok := as.actions.LoadOrStore(hash, &actionMsg{})
	if ok {
		log.L().Debug("Action already requested", log.Hex("hash", hash[:]))
		return
	}
	log.L().Debug("Requesting action", log.Hex("hash", hash[:]))
	as.trigger(hash)
	return
}

// ReceiveAction receives an action
func (as *ActionSync) ReceiveAction(_ context.Context, hash hash.Hash256) {
	if !as.IsReady() {
		return
	}
	log.L().Debug("received action", log.Hex("hash", hash[:]))
	as.actions.Delete(hash)
}

func (as *ActionSync) sync() {
	defer as.wg.Done()
	for {
		select {
		case hash := <-as.syncChan:
			log.L().Debug("syncing action", log.Hex("hash", hash[:]))
			channelFullnessMtc.WithLabelValues("action").Set(float64(len(as.syncChan)) / float64(cap(as.syncChan)))
			ctx, cancel := context.WithTimeout(context.Background(), unicaseTimeout)
			defer cancel()
			msg, ok := as.actions.Load(hash)
			if !ok {
				log.L().Debug("action not requested or already received", log.Hex("hash", hash[:]))
				continue
			}
			if time.Since(msg.(*actionMsg).lastTime) < as.cfg.Interval {
				log.L().Debug("action is recently requested", log.Hex("hash", hash[:]))
				continue
			}
			msg.(*actionMsg).lastTime = time.Now()
			// TODO: enhancement, request multiple actions in one message
			if err := as.requestFromNeighbors(ctx, hash); err != nil {
				log.L().Warn("Failed to request action from neighbors", zap.Error(err))
				counterMtc.WithLabelValues("failed").Inc()
			}
		case <-as.quit:
			log.L().Info("quitting action sync")
			return
		}
	}
}

func (as *ActionSync) triggerSync() {
	defer as.wg.Done()
	ticker := time.NewTicker(as.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			as.actions.Range(func(key, value interface{}) bool {
				as.trigger(key.(hash.Hash256))
				return true
			})
		case <-as.quit:
			log.L().Info("quitting action trigger sync")
			return
		}
	}
}

func (as *ActionSync) trigger(hash hash.Hash256) {
	if !as.IsReady() {
		return
	}

	select {
	case as.syncChan <- hash:
		channelFullnessMtc.WithLabelValues("action").Set(float64(len(as.syncChan)) / float64(cap(as.syncChan)))
	default:
		log.L().Warn("action sync channel is full, fail to sync action", log.Hex("hash", hash[:]))
	}
}

func (as *ActionSync) selectPeers() ([]peer.AddrInfo, error) {
	neighbors, err := as.helper.P2PNeighbor()
	if err != nil {
		return nil, err
	}
	repeat := batchPeerSize
	if repeat > len(neighbors) {
		repeat = len(neighbors)
	}
	if repeat == 0 {
		return nil, errors.New("no peers")
	}
	peers := make([]peer.AddrInfo, repeat)
	for i := 0; i < repeat; i++ {
		peer := neighbors[fastrand.Uint32n(uint32(len(neighbors)))]
		peers[i] = peer
	}
	return peers, nil
}

func (as *ActionSync) requestFromNeighbors(ctx context.Context, hash hash.Hash256) error {
	l := log.L().With(log.Hex("hash", hash[:]))
	neighbors, err := as.selectPeers()
	if err != nil {
		l.Debug("Failed to get neighbors", zap.Error(err))
		return err
	}
	success := false
	for i := range neighbors {
		if err := as.helper.UnicastOutbound(ctx, neighbors[i], &iotexrpc.ActionSync{Hashes: [][]byte{hash[:]}}); err != nil {
			l.Debug("Failed to request action", zap.Error(err), zap.String("peer", neighbors[i].String()))
			continue
		}
		success = true
	}
	if !success {
		l.Debug("Failed to request action from neighbors", zap.Any("peers", neighbors))
		return errors.Errorf("failed to request action from neighbors")
	}
	return nil
}
