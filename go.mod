module github.com/iotexproject/iotex-core

go 1.13

require (
	github.com/btcsuite/btcd v0.20.1-beta // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/ethereum/go-ethereum v1.9.5
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a
	github.com/go-logfmt/logfmt v0.5.0 // indirect
	github.com/go-sql-driver/mysql v1.4.1
	github.com/gogo/protobuf v1.3.1 // indirect
	github.com/golang/mock v1.4.4
	github.com/golang/protobuf v1.4.3
	github.com/golang/snappy v0.0.2-0.20200707131729-196ae77b8a26
	github.com/gorilla/websocket v1.4.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/iotexproject/go-fsm v1.0.0
	github.com/iotexproject/go-p2p v0.2.12
	github.com/iotexproject/go-pkgs v0.1.5-0.20210105202208-2dc9b27250a6
	github.com/iotexproject/iotex-address v0.2.4
	github.com/iotexproject/iotex-antenna-go/v2 v2.4.2-0.20201211202736-96d536a425fe
	github.com/iotexproject/iotex-election v0.3.5-0.20201031050050-c3ab4f339a54
	github.com/iotexproject/iotex-proto v0.5.0
	github.com/libp2p/go-libp2p v0.0.21 // indirect
	github.com/libp2p/go-libp2p-peerstore v0.0.5
	github.com/mattn/go-sqlite3 v1.11.0
	github.com/miguelmota/go-ethereum-hdwallet v0.0.0-20200123000308-a60dcd172b4c
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1
	github.com/minio/sha256-simd v0.1.1 // indirect
	github.com/multiformats/go-multiaddr v0.0.2
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.3.0
	github.com/rodaine/table v1.0.1
	github.com/rs/zerolog v1.18.0
	github.com/schollz/progressbar/v2 v2.15.0
	github.com/spf13/cobra v1.1.1
	github.com/stretchr/testify v1.6.1
	github.com/tyler-smith/go-bip39 v1.0.2
	go.elastic.co/ecszap v1.0.0
	go.etcd.io/bbolt v1.3.5
	go.uber.org/automaxprocs v1.2.0
	go.uber.org/config v1.3.1
	go.uber.org/zap v1.14.0
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/net v0.0.0-20201021035429-f5854403a974
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9
	google.golang.org/genproto v0.0.0-20201211151036-40ec1c210f7a
	google.golang.org/grpc v1.33.1
	google.golang.org/protobuf v1.25.0
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/ethereum/go-ethereum => github.com/iotexproject/go-ethereum v0.3.1

replace golang.org/x/xerrors => golang.org/x/xerrors v0.0.0-20190212162355-a5947ffaace3
