module github.com/iotexproject/iotex-core

go 1.13

require (
	github.com/armon/consul-api v0.0.0-20180202201655-eb2c6b5be1b6 // indirect
	github.com/btcsuite/btcd v0.20.1-beta // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/coreos/go-etcd v2.0.0+incompatible // indirect
	github.com/cpuguy83/go-md2man v1.0.10 // indirect
	github.com/ethereum/go-ethereum v1.9.5
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a
	github.com/go-logfmt/logfmt v0.5.0 // indirect
	github.com/go-sql-driver/mysql v1.4.1
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/golang/groupcache v0.0.0-20191027212112-611e8accdfc9
	github.com/golang/mock v1.4.4
	github.com/golang/protobuf v1.4.2
	github.com/golang/snappy v0.0.1
	github.com/gorilla/websocket v1.4.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/iotexproject/go-fsm v1.0.0
	github.com/iotexproject/go-p2p v0.2.12
	github.com/iotexproject/go-pkgs v0.1.2-0.20200523040337-5f1d9ddaa8ee
	github.com/iotexproject/iotex-address v0.2.2
	github.com/iotexproject/iotex-antenna-go/v2 v2.4.0
	github.com/iotexproject/iotex-election v0.3.5-0.20201031050050-c3ab4f339a54
	github.com/iotexproject/iotex-proto v0.4.4-0.20201029172022-a8466422b0f1
	github.com/libp2p/go-libp2p v0.0.21 // indirect
	github.com/libp2p/go-libp2p-peerstore v0.0.5
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/miguelmota/go-ethereum-hdwallet v0.0.0-20200123000308-a60dcd172b4c
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1
	github.com/minio/sha256-simd v0.1.1 // indirect
	github.com/multiformats/go-multiaddr v0.0.2
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.3.0
	github.com/rs/zerolog v1.18.0
	github.com/schollz/progressbar/v2 v2.15.0
	github.com/spf13/cobra v1.1.1
	github.com/stretchr/testify v1.5.1
	github.com/syndtr/goleveldb v1.0.1-0.20190923125748-758128399b1d // indirect
	github.com/tyler-smith/go-bip39 v1.0.2
	github.com/ugorji/go/codec v0.0.0-20181204163529-d75b2dcb6bc8 // indirect
	github.com/xordataexchange/crypt v0.0.3-0.20170626215501-b2862e3d0a77 // indirect
	go.etcd.io/bbolt v1.3.5
	go.uber.org/automaxprocs v1.2.0
	go.uber.org/config v1.3.1
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/net v0.0.0-20200707034311-ab3426394381
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
	google.golang.org/genproto v0.0.0-20200526211855-cb27e3aa2013
	google.golang.org/grpc v1.28.0
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v2 v2.2.8
)

replace github.com/ethereum/go-ethereum => github.com/iotexproject/go-ethereum v0.3.1

replace golang.org/x/xerrors => golang.org/x/xerrors v0.0.0-20190212162355-a5947ffaace3