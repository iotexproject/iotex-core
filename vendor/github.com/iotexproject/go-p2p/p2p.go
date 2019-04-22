package p2p

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/time/rate"

	lru "github.com/hashicorp/golang-lru"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	relay "github.com/libp2p/go-libp2p-circuit"
	connmgr "github.com/libp2p/go-libp2p-connmgr"
	crypto "github.com/libp2p/go-libp2p-crypto"
	discovery "github.com/libp2p/go-libp2p-discovery"
	host "github.com/libp2p/go-libp2p-host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	net "github.com/libp2p/go-libp2p-net"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	protocol "github.com/libp2p/go-libp2p-protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	stream "github.com/libp2p/go-libp2p-transport-upgrader"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
	sm_yamux "github.com/whyrusleeping/go-smux-yamux"
	"go.uber.org/zap"
)

func init() {
	multiaddr.SwapToP2pMultiaddrs()
}

// HandleBroadcast defines the callback function triggered when a broadcast message reaches a host
type HandleBroadcast func(ctx context.Context, data []byte) error

// HandleUnicast defines the callback function triggered when a unicast message reaches a host
type HandleUnicast func(ctx context.Context, w io.Writer, data []byte) error

// Config enumerates the configs required by a host
type Config struct {
	HostName                 string          `yaml:"hostName"`
	Port                     int             `yaml:"port"`
	ExternalHostName         string          `yaml:"externalHostName"`
	ExternalPort             int             `yaml:"externalPort"`
	SecureIO                 bool            `yaml:"secureIO"`
	Gossip                   bool            `yaml:"gossip"`
	ConnectTimeout           time.Duration   `yaml:"connectTimeout"`
	MasterKey                string          `yaml:"masterKey"`
	Relay                    string          `yaml:"relay"` // could be `active`, `nat`, `disable`
	ConnLowWater             int             `yaml:"connLowWater"`
	ConnHighWater            int             `yaml:"connHighWater"`
	RateLimiterLRUSize       int             `yaml:"rateLimiterLRUSize"`
	BlackListLRUSize         int             `yaml:"blackListLRUSize"`
	BlackListCleanupInterval time.Duration   `yaml:"blackListCleanupInterval"`
	ConnGracePeriod          time.Duration   `yaml:"connGracePeriod"`
	EnableRateLimit          bool            `yaml:"enableRateLimit"`
	RateLimit                RateLimitConfig `yaml:"rateLimit"`
}

// RateLimitConfig all numbers are per second value.
type RateLimitConfig struct {
	GlobalUnicastAvg   int `yaml:"globalUnicastAvg"`
	GlobalUnicastBurst int `yaml:"globalUnicastBurst"`
	PeerAvg            int `yaml:"peerAvg"`
	PeerBurst          int `yaml:"peerBurst"`
}

// ProtocolDHT is the DHT protocol ID
var ProtocolDHT protocol.ID = "/iotex/kad/1.0.0"

// DefaultConfig is a set of default configs
var DefaultConfig = Config{
	HostName:                 "127.0.0.1",
	Port:                     30001,
	ExternalHostName:         "",
	ExternalPort:             30001,
	SecureIO:                 false,
	Gossip:                   false,
	ConnectTimeout:           time.Minute,
	MasterKey:                "",
	Relay:                    "disable",
	ConnLowWater:             200,
	ConnHighWater:            500,
	RateLimiterLRUSize:       1000,
	BlackListLRUSize:         1000,
	BlackListCleanupInterval: 600 * time.Second,
	ConnGracePeriod:          0,
	EnableRateLimit:          false,
	RateLimit:                DefaultRatelimitConfig,
}

// DefaultRatelimitConfig is the default rate limit config
var DefaultRatelimitConfig = RateLimitConfig{
	GlobalUnicastAvg:   300,
	GlobalUnicastBurst: 500,
	PeerAvg:            300,
	PeerBurst:          500,
}

// Option defines the option function to modify the config for a host
type Option func(cfg *Config) error

// HostName is the option to override the host name or IP address
func HostName(hostName string) Option {
	return func(cfg *Config) error {
		cfg.HostName = hostName
		return nil
	}
}

// Port is the option to override the port number
func Port(port int) Option {
	return func(cfg *Config) error {
		cfg.Port = port
		return nil
	}
}

// ExternalHostName is the option to set the host name or IP address seen from external
func ExternalHostName(externalHostName string) Option {
	return func(cfg *Config) error {
		cfg.ExternalHostName = externalHostName
		return nil
	}
}

// ExternalPort is the option to set the port number seen from external
func ExternalPort(externalPort int) Option {
	return func(cfg *Config) error {
		cfg.ExternalPort = externalPort
		return nil
	}
}

// SecureIO is to indicate using secured I/O
func SecureIO() Option {
	return func(cfg *Config) error {
		cfg.SecureIO = true
		return nil
	}
}

// Gossip is to indicate using gossip protocol
func Gossip() Option {
	return func(cfg *Config) error {
		cfg.Gossip = true
		return nil
	}
}

// ConnectTimeout is the option to override the connect timeout
func ConnectTimeout(timout time.Duration) Option {
	return func(cfg *Config) error {
		cfg.ConnectTimeout = timout
		return nil
	}
}

// MasterKey is to determine network identifier
func MasterKey(masterKey string) Option {
	return func(cfg *Config) error {
		cfg.MasterKey = masterKey
		return nil
	}
}

// WithRateLimit is to indicate limiting msg rate from peers
func WithRateLimit(rcfg RateLimitConfig) Option {
	return func(cfg *Config) error {
		cfg.EnableRateLimit = true
		cfg.RateLimit = rcfg
		return nil
	}
}

// WithRelay config relay option.
func WithRelay(relayType string) Option {
	return func(cfg *Config) error {
		cfg.Relay = relayType
		return nil
	}
}

// WithConnectionManagerConfig set configuration for connection manager.
func WithConnectionManagerConfig(lo, hi int, grace time.Duration) Option {
	return func(cfg *Config) error {
		cfg.ConnLowWater = lo
		cfg.ConnHighWater = hi
		cfg.ConnGracePeriod = grace
		return nil
	}
}

// Host is the main struct that represents a host that communicating with the rest of the P2P networks
type Host struct {
	host           host.Host
	cfg            Config
	topics         map[string]interface{}
	kad            *dht.IpfsDHT
	kadKey         cid.Cid
	newPubSub      func(ctx context.Context, h host.Host, opts ...pubsub.Option) (*pubsub.PubSub, error)
	pubs           map[string]*pubsub.PubSub
	blacklists     map[string]*LRUBlacklist
	subs           map[string]*pubsub.Subscription
	close          chan interface{}
	ctx            context.Context
	peersLimiters  *lru.Cache
	unicastLimiter *rate.Limiter
}

// NewHost constructs a host struct
func NewHost(ctx context.Context, options ...Option) (*Host, error) {
	cfg := DefaultConfig
	for _, option := range options {
		if err := option(&cfg); err != nil {
			return nil, err
		}
	}
	ip, err := EnsureIPv4(cfg.HostName)
	if err != nil {
		return nil, err
	}
	masterKey := cfg.MasterKey
	// If ID is not given use network address instead
	if masterKey == "" {
		masterKey = fmt.Sprintf("%s:%d", ip, cfg.Port)
	}
	sk, _, err := generateKeyPair(masterKey)
	if err != nil {
		return nil, err
	}
	var extMultiAddr multiaddr.Multiaddr
	// Set external address and replace private key it external host name is given
	if cfg.ExternalHostName != "" {
		extIP, err := EnsureIPv4(cfg.ExternalHostName)
		if err != nil {
			return nil, err
		}
		masterKey := cfg.MasterKey
		// If ID is not given use network address instead
		if masterKey == "" {
			masterKey = fmt.Sprintf("%s:%d", cfg.ExternalHostName, cfg.ExternalPort)
		}
		sk, _, err = generateKeyPair(masterKey)
		if err != nil {
			return nil, err
		}
		extMultiAddr, err = multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", extIP, cfg.ExternalPort))
		if err != nil {
			return nil, err
		}
	}
	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", ip, cfg.Port)),
		libp2p.AddrsFactory(func(addrs []multiaddr.Multiaddr) []multiaddr.Multiaddr {
			if extMultiAddr != nil {
				return append(addrs, extMultiAddr)
			}
			return addrs
		}),
		libp2p.Identity(sk),
		libp2p.Transport(func(upgrader *stream.Upgrader) *tcp.TcpTransport {
			return &tcp.TcpTransport{Upgrader: upgrader, ConnectTimeout: cfg.ConnectTimeout}
		}),
		libp2p.Muxer("/yamux/2.0.0", sm_yamux.DefaultTransport),
		libp2p.ConnectionManager(connmgr.NewConnManager(cfg.ConnLowWater, cfg.ConnHighWater, cfg.ConnGracePeriod)),
	}
	if !cfg.SecureIO {
		opts = append(opts, libp2p.NoSecurity)
	}

	// relay option
	if cfg.Relay == "active" {
		opts = append(opts, libp2p.EnableRelay(relay.OptActive, relay.OptHop))
	} else if cfg.Relay == "nat" {
		opts = append(opts, libp2p.EnableRelay(), libp2p.NATPortMap())
	} else {
		opts = append(opts, libp2p.DisableRelay())
	}

	host, err := libp2p.New(ctx, opts...)
	if err != nil {
		return nil, err
	}
	kad, err := dht.New(ctx, host, dhtopts.Protocols(ProtocolDHT))
	if err != nil {
	}
	if err := kad.Bootstrap(ctx); err != nil {
		return nil, err
	}
	newPubSub := pubsub.NewFloodSub
	if cfg.Gossip {
		newPubSub = pubsub.NewGossipSub
	}
	v1b := cid.V1Builder{Codec: cid.Raw, MhType: multihash.SHA2_256}
	cid, err := v1b.Sum([]byte(masterKey))
	if err != nil {
		return nil, err
	}
	limiters, err := lru.New(cfg.RateLimiterLRUSize)
	if err != nil {
		return nil, err
	}
	myHost := Host{
		host:           host,
		cfg:            cfg,
		topics:         make(map[string]interface{}),
		kad:            kad,
		kadKey:         cid,
		newPubSub:      newPubSub,
		pubs:           make(map[string]*pubsub.PubSub),
		blacklists:     make(map[string]*LRUBlacklist),
		subs:           make(map[string]*pubsub.Subscription),
		close:          make(chan interface{}),
		ctx:            ctx,
		peersLimiters:  limiters,
		unicastLimiter: rate.NewLimiter(rate.Limit(cfg.RateLimit.GlobalUnicastAvg), cfg.RateLimit.GlobalUnicastBurst),
	}

	addrs := make([]string, 0)
	for _, ma := range myHost.Addresses() {
		addrs = append(addrs, ma.String())
	}
	Logger().Info("P2p host started.",
		zap.Strings("address", addrs),
		zap.Bool("secureIO", myHost.cfg.SecureIO),
		zap.Bool("gossip", myHost.cfg.Gossip))
	return &myHost, nil
}

// JoinOverlay triggers the host to join the DHT overlay
func (h *Host) JoinOverlay(ctx context.Context) {
	routingDiscovery := discovery.NewRoutingDiscovery(h.kad)
	discovery.Advertise(ctx, routingDiscovery, h.kadKey.String())
}

// AddUnicastPubSub adds a unicast topic that the host will pay attention to
func (h *Host) AddUnicastPubSub(topic string, callback HandleUnicast) error {
	if _, ok := h.topics[topic]; ok {
		return nil
	}
	h.host.SetStreamHandler(protocol.ID(topic), func(stream net.Stream) {
		defer func() {
			if err := stream.Close(); err != nil {
				Logger().Error("Error when closing a unicast stream.", zap.Error(err))
			}
		}()
		if h.cfg.EnableRateLimit && !h.unicastLimiter.Allow() {
			Logger().Warn("Drop unicast sream due to high traffic volume.")
			return
		}
		/*
			src := stream.Conn().RemotePeer()
			allowed, err := h.allowSource(src)
			if err != nil {
				Logger().Error("Error when checking if the source is allowed.", zap.Error(err))
				return
			}
			if !allowed {
				// TODO: blacklist src for unicast too
				return
			}
		*/
		data, err := ioutil.ReadAll(stream)
		if err != nil {
			Logger().Error("Error when subscribing a unicast message.", zap.Error(err))
			return
		}
		ctx := context.WithValue(context.Background(), unicastCtxKey{}, stream)
		if err := callback(ctx, stream, data); err != nil {
			Logger().Error("Error when processing a unicast message.", zap.Error(err))
		}
	})
	h.topics[topic] = nil
	return nil
}

// AddBroadcastPubSub adds a broadcast topic that the host will pay attention to. This need to be called before using
// Connect/JoinOverlay. Otherwise, pubsub may not be aware of the existing overlay topology
func (h *Host) AddBroadcastPubSub(topic string, callback HandleBroadcast) error {
	if _, ok := h.pubs[topic]; ok {
		return nil
	}
	blacklist, err := NewLRUBlacklist(h.cfg.BlackListLRUSize)
	if err != nil {
		return err
	}
	pub, err := h.newPubSub(
		h.ctx,
		h.host,
		pubsub.WithMessageSigning(true),
		pubsub.WithStrictSignatureVerification(true),
		pubsub.WithBlacklist(blacklist),
	)
	if err != nil {
		return err
	}
	sub, err := pub.Subscribe(topic)
	if err != nil {
		return err
	}
	h.pubs[topic] = pub
	h.blacklists[topic] = blacklist
	h.subs[topic] = sub
	go func() {
		for {
			select {
			case <-h.close:
				return
			default:
				ctx := context.Background()
				msg, err := sub.Next(ctx)
				if err != nil {
					Logger().Error("Error when subscribing a broadcast message.", zap.Error(err))
					continue
				}
				src := msg.GetFrom()
				allowed, err := h.allowSource(src)
				if err != nil {
					Logger().Error("Error when checking if the source is allowed.", zap.Error(err))
					continue
				}
				if !allowed {
					h.blacklists[topic].Add(src)
					Logger().Warn("Blacklist a peer", zap.Any("id", src))
					continue
				}
				h.blacklists[topic].Remove(src)
				ctx = context.WithValue(ctx, broadcastCtxKey{}, msg)
				if err := callback(ctx, msg.Data); err != nil {
					Logger().Error("Error when processing a broadcast message.", zap.Error(err))
				}
			}
		}
	}()
	go func() {
		for {
			time.Sleep(h.cfg.BlackListCleanupInterval)
			h.blacklists[topic].RemoveOldest()
		}
	}()
	return nil
}

// ConnectWithMultiaddr connects a peer given the multi address
func (h *Host) ConnectWithMultiaddr(ctx context.Context, ma multiaddr.Multiaddr) error {
	target, err := peerstore.InfoFromP2pAddr(ma)
	if err != nil {
		return err
	}
	if err := h.host.Connect(ctx, *target); err != nil {
		return err
	}
	Logger().Debug(
		"P2P peer connected.",
		zap.String("multiAddress", ma.String()),
	)
	return nil
}

// Connect connects a peer.
func (h *Host) Connect(ctx context.Context, target peerstore.PeerInfo) error {
	if err := h.host.Connect(ctx, target); err != nil {
		return err
	}
	Logger().Debug(
		"P2P peer connected.",
		zap.String("peer", fmt.Sprintf("%+v", target)),
	)
	return nil
}

// Broadcast sends a message to the hosts who subscribe the topic
func (h *Host) Broadcast(topic string, data []byte) error {
	pub, ok := h.pubs[topic]
	if !ok {
		return nil
	}
	return pub.Publish(topic, data)
}

// Unicast sends a message to a peer on the given address
func (h *Host) Unicast(ctx context.Context, target peerstore.PeerInfo, topic string, data []byte) error {
	if err := h.Connect(ctx, target); err != nil {
		return err
	}
	stream, err := h.host.NewStream(ctx, target.ID, protocol.ID(topic))
	if err != nil {
		return err
	}
	defer func() { err = stream.Close() }()
	if _, err = stream.Write(data); err != nil {
		return err
	}
	return nil
}

// HostIdentity returns the host identity string
func (h *Host) HostIdentity() string { return h.host.ID().Pretty() }

// OverlayIdentity returns the overlay identity string
func (h *Host) OverlayIdentity() string { return h.kadKey.String() }

// Addresses returns the multi address
func (h *Host) Addresses() []multiaddr.Multiaddr {
	hostID, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ipfs/%s", h.HostIdentity()))
	addrs := make([]multiaddr.Multiaddr, 0)
	for _, addr := range h.host.Addrs() {
		addrs = append(addrs, addr.Encapsulate(hostID))
	}
	return addrs
}

// Info returns host's perr info.
func (h *Host) Info() peerstore.PeerInfo {
	return peerstore.PeerInfo{ID: h.host.ID(), Addrs: h.host.Addrs()}
}

// Neighbors returns the closest peer addresses
func (h *Host) Neighbors(ctx context.Context) ([]peerstore.PeerInfo, error) {
	peers := h.host.Peerstore().Peers()
	dedupedPeers := make(map[string]peer.ID)
	for _, p := range peers {
		idStr := p.Pretty()
		if idStr == h.host.ID().Pretty() || idStr == "" {
			continue
		}
		dedupedPeers[idStr] = p
	}
	neighbors := make([]peerstore.PeerInfo, 0)
	for _, p := range dedupedPeers {
		neighbors = append(neighbors, h.kad.FindLocal(p))
	}
	return neighbors, nil
}

// Close closes the host
func (h *Host) Close() error {
	close(h.close)
	for _, sub := range h.subs {
		sub.Cancel()
	}
	if err := h.kad.Close(); err != nil {
		return err
	}
	if err := h.host.Close(); err != nil {
		return err
	}
	return nil
}

func (h *Host) allowSource(src peer.ID) (bool, error) {
	if !h.cfg.EnableRateLimit {
		return true, nil
	}
	var limiter *rate.Limiter
	val, ok := h.peersLimiters.Get(src)
	if ok {
		limiter, ok = val.(*rate.Limiter)
		if !ok {
			return false, errors.New("error when casting to limiter struct")
		}
	} else {
		limiter = rate.NewLimiter(rate.Limit(h.cfg.RateLimit.PeerAvg), h.cfg.RateLimit.PeerBurst)
		h.peersLimiters.Add(src, limiter)
	}
	return limiter.Allow(), nil
}

// generateKeyPair generates the public key and private key by network address
func generateKeyPair(masterKey string) (crypto.PrivKey, crypto.PubKey, error) {
	hash := sha1.Sum([]byte(masterKey))
	seedBytes := hash[12:]
	seedBytes[0] = 0
	seed := int64(binary.BigEndian.Uint64(seedBytes))
	r := rand.New(rand.NewSource(seed))
	return crypto.GenerateKeyPairWithReader(crypto.Ed25519, 2048, r)
}
