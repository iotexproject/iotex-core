FROM golang:1.10.2-stretch

WORKDIR $GOPATH/src/github.com/iotexproject/iotex-core/

RUN apt-get install -y --no-install-recommends make

COPY . .

ARG SKIP_DEP=false

RUN if [ "$SKIP_DEP" != true ] ; \
    then \
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh && \
        dep ensure --vendor-only; \
    fi

RUN mkdir -p $GOPATH/src/github.com/CoderZhi/go-ethereum/ && \
    mkdir -p $GOPATH/pkg/linux_amd64/github.com/CoderZhi/ && \
    rm -rf ./vendor/github.com/CoderZhi/go-ethereum/ && \
    tar -xzvf ./pkg/go-ethereum.tar.gz -C ./pkg && \
    cp -r ./pkg/go-ethereum/binary_linux/* $GOPATH/pkg/linux_amd64/github.com/CoderZhi/ && \
    cp -r ./pkg/go-ethereum/go-ethereum/* $GOPATH/src/github.com/CoderZhi/go-ethereum/ && \
    rm -rf ./pkg/go-ethereum/ && \
    make clean build && \
    ln -s $GOPATH/src/github.com/iotexproject/iotex-core/bin/server /usr/local/bin/iotex-server  && \
    ln -s $GOPATH/src/github.com/iotexproject/iotex-core/bin/actioninjector /usr/local/bin/iotex-actioninjector && \
    ln -s $GOPATH/src/github.com/iotexproject/iotex-core/bin/addrgen /usr/local/bin/iotex-addrgen && \
    cp ./crypto/lib/libsect283k1_ubuntu.so /usr/lib/ && \
    cp ./crypto/lib/blslib/libtblsmnt_ubuntu.so /usr/lib/ && \
    mkdir -p /etc/iotex/ && \
    ln -s $GOPATH/src/github.com/iotexproject/iotex-core/blockchain/testnet_actions.yaml /etc/iotex/testnet_actions.yaml && \
    cp ./e2etest/config_local_delegate.yaml /etc/iotex/config_local_delegate.yaml && \
    cp ./e2etest/config_local_fullnode.yaml /etc/iotex/config_local_fullnode.yaml

CMD [ "iotex-server", "-config-path=/etc/iotex/config_local_fullnode.yaml"]
