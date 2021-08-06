module github.com/iotexproject/iotex-core

go 1.13

require (
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/ethereum/go-ethereum v1.10.1
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a
	github.com/go-sql-driver/mysql v1.4.1
	github.com/golang/mock v1.4.4
	github.com/golang/protobuf v1.4.3
	github.com/golang/snappy v0.0.2-0.20200707131729-196ae77b8a26
	github.com/gorilla/websocket v1.4.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/iotexproject/go-fsm v1.0.0
	github.com/iotexproject/go-p2p v0.3.0
	github.com/iotexproject/go-pkgs v0.1.5
	github.com/iotexproject/iotex-address v0.2.4
	github.com/iotexproject/iotex-antenna-go/v2 v2.5.1-0.20210604061028-2c2056a5bfdb
	github.com/iotexproject/iotex-election v0.3.5-0.20210611041425-20ddf674363d
	github.com/iotexproject/iotex-proto v0.5.2
	github.com/libp2p/go-libp2p-core v0.8.5
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/miguelmota/go-ethereum-hdwallet v0.0.0-20200123000308-a60dcd172b4c
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1
	github.com/multiformats/go-multiaddr v0.3.3
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.10.0
	github.com/rodaine/table v1.0.1
	github.com/rs/zerolog v1.18.0
	github.com/schollz/progressbar/v2 v2.15.0
	github.com/spf13/cobra v1.1.1
	github.com/stretchr/testify v1.7.0
	github.com/tyler-smith/go-bip39 v1.0.2
	go.elastic.co/ecszap v1.0.0
	go.etcd.io/bbolt v1.3.5
	go.uber.org/automaxprocs v1.2.0
	go.uber.org/config v1.3.1
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	golang.org/x/net v0.0.0-20210423184538-5f58ad60dda6
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/genproto v0.0.0-20201211151036-40ec1c210f7a
	google.golang.org/grpc v1.33.2
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/ethereum/go-ethereum => github.com/iotexproject/go-ethereum v1.7.4-0.20210604055808-21e763469056

replace golang.org/x/xerrors => golang.org/x/xerrors v0.0.0-20190212162355-a5947ffaace3
