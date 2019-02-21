FROM golang:1.11.5-stretch

WORKDIR $GOPATH/src/github.com/iotexproject/iotex-core/

RUN apt-get install -y --no-install-recommends make

COPY . .

RUN mkdir -p $GOPATH/pkg/linux_amd64/github.com/iotexproject/ && \
    make clean build && \
    cp $GOPATH/src/github.com/iotexproject/iotex-core/bin/server /usr/local/bin/iotex-server  && \
    cp $GOPATH/src/github.com/iotexproject/iotex-core/bin/actioninjector /usr/local/bin/iotex-actioninjector && \
    cp $GOPATH/src/github.com/iotexproject/iotex-core/bin/actioninjectorv2 /usr/local/bin/iotex-actioninjectorv2 && \
    cp $GOPATH/src/github.com/iotexproject/iotex-core/bin/addrgen /usr/local/bin/iotex-addrgen && \
    cp ./crypto/lib/libsect283k1_ubuntu.so /usr/lib/ && \
    cp ./crypto/lib/blslib/libtblsmnt_ubuntu.so /usr/lib/ && \
    mkdir -p /etc/iotex/ && \
    cp $GOPATH/src/github.com/iotexproject/iotex-core/blockchain/testnet_actions.yaml /etc/iotex/testnet_actions.yaml && \
    cp ./e2etest/config_local_delegate.yaml /etc/iotex/config_local_delegate.yaml && \
    cp ./e2etest/config_local_fullnode.yaml /etc/iotex/config_local_fullnode.yaml && \
    rm -rf $GOPATH/src/github.com/iotexproject/iotex-core/

CMD [ "iotex-server", "-config-path=/etc/iotex/config_local_fullnode.yaml"]
