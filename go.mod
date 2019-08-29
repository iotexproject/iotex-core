module github.com/iotexproject/iotex-core

go 1.12

require (
	github.com/cenkalti/backoff v2.1.1+incompatible
	github.com/ethereum/go-ethereum v1.8.27
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a
	github.com/go-logfmt/logfmt v0.4.0 // indirect
	github.com/go-sql-driver/mysql v1.4.1
	github.com/gogo/protobuf v1.2.1
	github.com/golang/groupcache v0.0.0-20190129154638-5b532d6fd5ef
	github.com/golang/mock v1.3.1
	github.com/golang/protobuf v1.3.1
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/iotexproject/go-fsm v1.0.0
	github.com/iotexproject/go-p2p v0.2.10
	github.com/iotexproject/go-pkgs v0.1.1-0.20190708233003-85a24189bbd4
	github.com/iotexproject/iotex-address v0.2.1-0.20190725020429-99e5492ea0c3
	github.com/iotexproject/iotex-antenna-go/v2 v2.3.0
	github.com/iotexproject/iotex-election v0.1.18-0.20190720010220-fddc58c26ff5
	github.com/iotexproject/iotex-proto v0.2.1-0.20190814190638-f74c55ffedf5
	github.com/libp2p/go-libp2p-connmgr v0.1.1 // indirect
	github.com/libp2p/go-libp2p-host v0.1.0 // indirect
	github.com/libp2p/go-libp2p-kad-dht v0.2.0 // indirect
	github.com/libp2p/go-libp2p-net v0.1.0 // indirect
	github.com/libp2p/go-libp2p-peer v0.2.0 // indirect
	github.com/libp2p/go-libp2p-peerstore v0.1.3
	github.com/libp2p/go-libp2p-protocol v0.1.0 // indirect
	github.com/libp2p/go-libp2p-pubsub v0.1.1 // indirect
	github.com/libp2p/go-stream-muxer v0.1.0 // indirect
	github.com/mattn/go-sqlite3 v1.10.0
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1
	github.com/multiformats/go-multiaddr v0.0.4
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.0.0
	github.com/rs/zerolog v1.14.3
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/testify v1.3.0
	github.com/whyrusleeping/go-smux-yamux v2.0.9+incompatible // indirect
	github.com/whyrusleeping/yamux v1.2.0 // indirect
	go.etcd.io/bbolt v1.3.2
	go.uber.org/automaxprocs v1.2.0
	go.uber.org/config v1.3.1
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20190618222545-ea8f1a30c443
	golang.org/x/net v0.0.0-20190620200207-3b0461eec859
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190712062909-fae7ac547cb7 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/appengine v1.5.0 // indirect
	google.golang.org/grpc v1.21.0
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/ethereum/go-ethereum => github.com/iotexproject/go-ethereum v0.2.1-0.20190723221211-a9fbf57d1cb7
