package dht

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	u "github.com/ipfs/go-ipfs-util"
	goprocess "github.com/jbenet/goprocess"
	periodicproc "github.com/jbenet/goprocess/periodic"
	peer "github.com/libp2p/go-libp2p-peer"
	routing "github.com/libp2p/go-libp2p-routing"
	multiaddr "github.com/multiformats/go-multiaddr"
)

var DefaultBootstrapPeers []multiaddr.Multiaddr

func init() {
	for _, s := range []string{
		"/dnsaddr/bootstrap.libp2p.io/ipfs/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/ipfs/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/ipfs/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		"/dnsaddr/bootstrap.libp2p.io/ipfs/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		"/ip4/104.131.131.82/tcp/4001/ipfs/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",            // mars.i.ipfs.io
		"/ip4/104.236.179.241/tcp/4001/ipfs/QmSoLPppuBtQSGwKDZT2M73ULpjvfd3aZ6ha4oFGL1KrGM",           // pluto.i.ipfs.io
		"/ip4/128.199.219.111/tcp/4001/ipfs/QmSoLSafTMBsPKadTEgaXctDQVcqN88CNLHXMkTNwMKPnu",           // saturn.i.ipfs.io
		"/ip4/104.236.76.40/tcp/4001/ipfs/QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzaDs64",             // venus.i.ipfs.io
		"/ip4/178.62.158.247/tcp/4001/ipfs/QmSoLer265NRgSp2LA3dPaeykiS1J6DifTC88f5uVQKNAd",            // earth.i.ipfs.io
		"/ip6/2604:a880:1:20::203:d001/tcp/4001/ipfs/QmSoLPppuBtQSGwKDZT2M73ULpjvfd3aZ6ha4oFGL1KrGM",  // pluto.i.ipfs.io
		"/ip6/2400:6180:0:d0::151:6001/tcp/4001/ipfs/QmSoLSafTMBsPKadTEgaXctDQVcqN88CNLHXMkTNwMKPnu",  // saturn.i.ipfs.io
		"/ip6/2604:a880:800:10::4a:5001/tcp/4001/ipfs/QmSoLV4Bbm51jM9C4gDYZQ9Cy3U6aXMJDAbzgu2fzaDs64", // venus.i.ipfs.io
		"/ip6/2a03:b0c0:0:1010::23:1001/tcp/4001/ipfs/QmSoLer265NRgSp2LA3dPaeykiS1J6DifTC88f5uVQKNAd", // earth.i.ipfs.io
	} {
		ma, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			panic(err)
		}
		DefaultBootstrapPeers = append(DefaultBootstrapPeers, ma)
	}
}

// BootstrapConfig specifies parameters used bootstrapping the DHT.
//
// Note there is a tradeoff between the bootstrap period and the
// number of queries. We could support a higher period with less
// queries.
type BootstrapConfig struct {
	Queries int           // how many queries to run per period
	Period  time.Duration // how often to run periodic bootstrap.
	Timeout time.Duration // how long to wait for a bootstrap query to run
}

var DefaultBootstrapConfig = BootstrapConfig{
	// For now, this is set to 1 query.
	// We are currently more interested in ensuring we have a properly formed
	// DHT than making sure our dht minimizes traffic. Once we are more certain
	// of our implementation's robustness, we should lower this down to 8 or 4.
	Queries: 1,

	// For now, this is set to 5 minutes, which is a medium period. We are
	// We are currently more interested in ensuring we have a properly formed
	// DHT than making sure our dht minimizes traffic.
	Period: time.Duration(5 * time.Minute),

	Timeout: time.Duration(10 * time.Second),
}

// Bootstrap ensures the dht routing table remains healthy as peers come and go.
// it builds up a list of peers by requesting random peer IDs. The Bootstrap
// process will run a number of queries each time, and run every time signal fires.
// These parameters are configurable.
//
// As opposed to BootstrapWithConfig, Bootstrap satisfies the routing interface
func (dht *IpfsDHT) Bootstrap(ctx context.Context) error {
	proc, err := dht.BootstrapWithConfig(DefaultBootstrapConfig)
	if err != nil {
		return err
	}

	// wait till ctx or dht.Context exits.
	// we have to do it this way to satisfy the Routing interface (contexts)
	go func() {
		defer proc.Close()
		select {
		case <-ctx.Done():
		case <-dht.Context().Done():
		}
	}()

	return nil
}

// BootstrapWithConfig ensures the dht routing table remains healthy as peers come and go.
// it builds up a list of peers by requesting random peer IDs. The Bootstrap
// process will run a number of queries each time, and run every time signal fires.
// These parameters are configurable.
//
// BootstrapWithConfig returns a process, so the user can stop it.
func (dht *IpfsDHT) BootstrapWithConfig(cfg BootstrapConfig) (goprocess.Process, error) {
	if cfg.Queries <= 0 {
		return nil, fmt.Errorf("invalid number of queries: %d", cfg.Queries)
	}

	proc := dht.Process().Go(func(p goprocess.Process) {
		<-p.Go(dht.bootstrapWorker(cfg)).Closed()
		for {
			select {
			case <-time.After(cfg.Period):
				<-p.Go(dht.bootstrapWorker(cfg)).Closed()
			case <-p.Closing():
				return
			}
		}
	})

	return proc, nil
}

// SignalBootstrap ensures the dht routing table remains healthy as peers come and go.
// it builds up a list of peers by requesting random peer IDs. The Bootstrap
// process will run a number of queries each time, and run every time signal fires.
// These parameters are configurable.
//
// SignalBootstrap returns a process, so the user can stop it.
func (dht *IpfsDHT) BootstrapOnSignal(cfg BootstrapConfig, signal <-chan time.Time) (goprocess.Process, error) {
	if cfg.Queries <= 0 {
		return nil, fmt.Errorf("invalid number of queries: %d", cfg.Queries)
	}

	if signal == nil {
		return nil, fmt.Errorf("invalid signal: %v", signal)
	}

	proc := periodicproc.Ticker(signal, dht.bootstrapWorker(cfg))

	return proc, nil
}

func (dht *IpfsDHT) bootstrapWorker(cfg BootstrapConfig) func(worker goprocess.Process) {
	return func(worker goprocess.Process) {
		// it would be useful to be able to send out signals of when we bootstrap, too...
		// maybe this is a good case for whole module event pub/sub?

		ctx := dht.Context()
		if err := dht.runBootstrap(ctx, cfg); err != nil {
			log.Warning(err)
			// A bootstrapping error is important to notice but not fatal.
		}
	}
}

// runBootstrap builds up list of peers by requesting random peer IDs
func (dht *IpfsDHT) runBootstrap(ctx context.Context, cfg BootstrapConfig) error {
	bslog := func(msg string) {
		log.Debugf("DHT %s dhtRunBootstrap %s -- routing table size: %d", dht.self, msg, dht.routingTable.Size())
	}
	bslog("start")
	defer bslog("end")
	defer log.EventBegin(ctx, "dhtRunBootstrap").Done()

	var merr u.MultiErr

	randomID := func() peer.ID {
		// 16 random bytes is not a valid peer id. it may be fine becuase
		// the dht will rehash to its own keyspace anyway.
		id := make([]byte, 16)
		rand.Read(id)
		id = u.Hash(id)
		return peer.ID(id)
	}

	// bootstrap sequentially, as results will compound
	runQuery := func(ctx context.Context, id peer.ID) {
		ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()

		p, err := dht.FindPeer(ctx, id)
		if err == routing.ErrNotFound {
			// this isn't an error. this is precisely what we expect.
		} else if err != nil {
			merr = append(merr, err)
		} else {
			// woah, actually found a peer with that ID? this shouldn't happen normally
			// (as the ID we use is not a real ID). this is an odd error worth logging.
			err := fmt.Errorf("Bootstrap peer error: Actually FOUND peer. (%s, %s)", id, p)
			log.Warningf("%s", err)
			merr = append(merr, err)
		}
	}

	// these should be parallel normally. but can make them sequential for debugging.
	// note that the core/bootstrap context deadline should be extended too for that.
	for i := 0; i < cfg.Queries; i++ {
		id := randomID()
		log.Debugf("Bootstrapping query (%d/%d) to random ID: %s", i+1, cfg.Queries, id)
		runQuery(ctx, id)
	}

	// Find self to distribute peer info to our neighbors.
	// Do this after bootstrapping.
	log.Debugf("Bootstrapping query to self: %s", dht.self)
	runQuery(ctx, dht.self)

	if len(merr) > 0 {
		return merr
	}
	return nil
}
