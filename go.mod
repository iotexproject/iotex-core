module github.com/iotexproject/iotex-core

go 1.17

require (
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/ethereum/go-ethereum v1.10.4
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a
	github.com/go-sql-driver/mysql v1.4.1
	github.com/golang/mock v1.4.4
	github.com/golang/protobuf v1.5.2
	github.com/golang/snappy v0.0.3
	github.com/gorilla/websocket v1.4.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/iotexproject/go-fsm v1.0.0
	github.com/iotexproject/go-p2p v0.2.12
	github.com/iotexproject/go-pkgs v0.1.6
	github.com/iotexproject/iotex-address v0.2.5
	github.com/iotexproject/iotex-antenna-go/v2 v2.5.1
	github.com/iotexproject/iotex-election v0.3.5-0.20210611041425-20ddf674363d
	github.com/iotexproject/iotex-proto v0.5.2
	github.com/libp2p/go-libp2p v0.0.21 // indirect
	github.com/libp2p/go-libp2p-peerstore v0.0.5
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/miguelmota/go-ethereum-hdwallet v0.1.1
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1
	github.com/minio/sha256-simd v0.1.1 // indirect
	github.com/multiformats/go-multiaddr v0.0.2
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.3.0
	github.com/rodaine/table v1.0.1
	github.com/rs/zerolog v1.18.0
	github.com/schollz/progressbar/v2 v2.15.0
	github.com/spf13/cobra v1.1.1
	github.com/stretchr/testify v1.7.0
	github.com/tyler-smith/go-bip39 v1.0.2
	go.elastic.co/ecszap v1.0.0
	go.etcd.io/bbolt v1.3.5
	go.opencensus.io v0.23.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.25.0
	go.opentelemetry.io/otel v1.0.1
	go.opentelemetry.io/otel/exporters/jaeger v1.0.1
	go.opentelemetry.io/otel/sdk v1.0.1
	go.opentelemetry.io/otel/trace v1.0.1
	go.uber.org/atomic v1.7.0 // indirect
	go.uber.org/automaxprocs v1.2.0
	go.uber.org/config v1.3.1
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/net v0.0.0-20210805182204-aaa1db679c0d
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/genproto v0.0.0-20201211151036-40ec1c210f7a
	google.golang.org/grpc v1.40.0
	google.golang.org/protobuf v1.26.0
	gopkg.in/yaml.v2 v2.4.0
)

require (
	github.com/StackExchange/wmi v0.0.0-20180116203802-5d049714c4a6 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/btcsuite/btcd v0.21.0-beta // indirect
	github.com/btcsuite/btcutil v1.0.3-0.20201208143702-a53e38424cce // indirect
	github.com/cespare/xxhash/v2 v2.1.1 // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/davidlazar/go-crypto v0.0.0-20170701192655-dcfb0a7ac018 // indirect
	github.com/deckarep/golang-set v1.7.1 // indirect
	github.com/dustinxie/gmsm v1.4.0 // indirect
	github.com/go-ole/go-ole v1.2.1 // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/google/uuid v1.1.5 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/hashicorp/golang-lru v0.5.5-0.20210104140557-80c98217689d // indirect
	github.com/holiman/uint256 v1.2.0 // indirect
	github.com/huin/goupnp v1.0.2 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/ipfs/go-cid v0.0.1 // indirect
	github.com/ipfs/go-datastore v0.0.5 // indirect
	github.com/ipfs/go-ipfs-util v0.0.1 // indirect
	github.com/ipfs/go-log v0.0.1 // indirect
	github.com/ipfs/go-todocounter v0.0.1 // indirect
	github.com/jackpal/gateway v1.0.5 // indirect
	github.com/jackpal/go-nat-pmp v1.0.2-0.20160603034137-1fa385a6f458 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jbenet/go-temp-err-catcher v0.0.0-20150120210811-aac704a3f4f2 // indirect
	github.com/jbenet/goprocess v0.0.0-20160826012719-b497e2f366b8 // indirect
	github.com/koron/go-ssdp v0.0.0-20180514024734-4a0ed625a78b // indirect
	github.com/libp2p/go-addr-util v0.0.1 // indirect
	github.com/libp2p/go-buffer-pool v0.0.2 // indirect
	github.com/libp2p/go-conn-security v0.0.1 // indirect
	github.com/libp2p/go-conn-security-multistream v0.0.2 // indirect
	github.com/libp2p/go-flow-metrics v0.0.1 // indirect
	github.com/libp2p/go-libp2p-autonat v0.0.4 // indirect
	github.com/libp2p/go-libp2p-circuit v0.0.4 // indirect
	github.com/libp2p/go-libp2p-connmgr v0.0.3 // indirect
	github.com/libp2p/go-libp2p-core v0.0.1 // indirect
	github.com/libp2p/go-libp2p-crypto v0.0.1 // indirect
	github.com/libp2p/go-libp2p-discovery v0.0.2 // indirect
	github.com/libp2p/go-libp2p-host v0.0.2 // indirect
	github.com/libp2p/go-libp2p-interface-connmgr v0.0.3 // indirect
	github.com/libp2p/go-libp2p-interface-pnet v0.0.1 // indirect
	github.com/libp2p/go-libp2p-kad-dht v0.0.10 // indirect
	github.com/libp2p/go-libp2p-kbucket v0.1.1 // indirect
	github.com/libp2p/go-libp2p-loggables v0.0.1 // indirect
	github.com/libp2p/go-libp2p-metrics v0.0.1 // indirect
	github.com/libp2p/go-libp2p-nat v0.0.4 // indirect
	github.com/libp2p/go-libp2p-net v0.0.2 // indirect
	github.com/libp2p/go-libp2p-peer v0.1.0 // indirect
	github.com/libp2p/go-libp2p-pnet v0.1.0 // indirect
	github.com/libp2p/go-libp2p-protocol v0.0.1 // indirect
	github.com/libp2p/go-libp2p-pubsub v0.0.1 // indirect
	github.com/libp2p/go-libp2p-record v0.0.1 // indirect
	github.com/libp2p/go-libp2p-routing v0.0.1 // indirect
	github.com/libp2p/go-libp2p-secio v0.0.3 // indirect
	github.com/libp2p/go-libp2p-swarm v0.0.3 // indirect
	github.com/libp2p/go-libp2p-transport v0.0.4 // indirect
	github.com/libp2p/go-libp2p-transport-upgrader v0.0.2 // indirect
	github.com/libp2p/go-maddr-filter v0.0.1 // indirect
	github.com/libp2p/go-mplex v0.0.1 // indirect
	github.com/libp2p/go-msgio v0.0.2 // indirect
	github.com/libp2p/go-nat v0.0.3 // indirect
	github.com/libp2p/go-reuseport v0.0.1 // indirect
	github.com/libp2p/go-reuseport-transport v0.0.2 // indirect
	github.com/libp2p/go-stream-muxer v0.0.1 // indirect
	github.com/libp2p/go-tcp-transport v0.0.2 // indirect
	github.com/libp2p/go-ws-transport v0.0.2 // indirect
	github.com/magefile/mage v1.9.0 // indirect
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/mr-tron/base58 v1.1.2 // indirect
	github.com/multiformats/go-base32 v0.0.3 // indirect
	github.com/multiformats/go-multiaddr-dns v0.0.2 // indirect
	github.com/multiformats/go-multiaddr-net v0.0.1 // indirect
	github.com/multiformats/go-multibase v0.0.1 // indirect
	github.com/multiformats/go-multihash v0.0.5 // indirect
	github.com/multiformats/go-multistream v0.0.2 // indirect
	github.com/opentracing/opentracing-go v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.1.0 // indirect
	github.com/prometheus/common v0.7.0 // indirect
	github.com/prometheus/procfs v0.0.8 // indirect
	github.com/rjeczalik/notify v0.9.2 // indirect
	github.com/shirou/gopsutil v3.21.4-0.20210419000835-c7a38de76ee5+incompatible // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tklauser/go-sysconf v0.3.5 // indirect
	github.com/tklauser/numcpus v0.2.2 // indirect
	github.com/whyrusleeping/base32 v0.0.0-20170828182744-c30ac30633cc // indirect
	github.com/whyrusleeping/go-keyspace v0.0.0-20160322163242-5b898ac5add1 // indirect
	github.com/whyrusleeping/go-logging v0.0.0-20170515211332-0457bb6b88fc // indirect
	github.com/whyrusleeping/go-notifier v0.0.0-20170827234753-097c5d47330f // indirect
	github.com/whyrusleeping/go-smux-multiplex v3.0.16+incompatible // indirect
	github.com/whyrusleeping/go-smux-multistream v2.0.2+incompatible // indirect
	github.com/whyrusleeping/go-smux-yamux v2.0.9+incompatible // indirect
	github.com/whyrusleeping/mafmt v1.2.8 // indirect
	github.com/whyrusleeping/multiaddr-filter v0.0.0-20160516205228-e903e4adabd7 // indirect
	github.com/whyrusleeping/timecache v0.0.0-20160911033111-cfcb2f1abfee // indirect
	github.com/whyrusleeping/yamux v1.1.5 // indirect
	go.uber.org/multierr v1.5.0 // indirect
	golang.org/x/sys v0.0.0-20210816183151-1e6c022a8912 // indirect
	golang.org/x/term v0.0.0-20201126162022-7de9c90e9dd1 // indirect
	golang.org/x/text v0.3.6 // indirect
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/appengine v1.6.5 // indirect
	gopkg.in/natefinch/npipe.v2 v2.0.0-20160621034901-c1b8fa8bdcce // indirect
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c // indirect
)

replace github.com/ethereum/go-ethereum => github.com/iotexproject/go-ethereum v0.4.0-safefix

replace golang.org/x/xerrors => golang.org/x/xerrors v0.0.0-20190212162355-a5947ffaace3
