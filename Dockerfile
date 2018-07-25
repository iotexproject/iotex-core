FROM golang:1.10.2-stretch

RUN apt-get install -y --no-install-recommends make && \
    mkdir -p $GOPATH/src/github.com/iotexproject/iotex-core/

COPY ./ $GOPATH/src/github.com/iotexproject/iotex-core/

ARG SKIP_DEP=false

RUN if [ "$SKIP_DEP" != true ] ; \
    then \
        cd $GOPATH/src/github.com/iotexproject/iotex-core/ && \
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh && \
        dep ensure ; \
    fi


RUN cd $GOPATH/src/github.com/iotexproject/iotex-core/ && \
    make clean build && \
    ln -s $GOPATH/src/github.com/iotexproject/iotex-core/bin/server /usr/local/bin/iotex-server  && \
    ln -s $GOPATH/src/github.com/iotexproject/iotex-core/bin/actioninjector /usr/local/bin/iotex-actioninjector && \
    mkdir -p /usr/local/lib/iotex/ && \
    ln -s $GOPATH/src/github.com/iotexproject/iotex-core/bin/addrgen /usr/local/bin/iotex-addrgen && \
    mkdir -p /usr/local/lib/iotex/ && \
    cp $GOPATH/src/github.com/iotexproject/iotex-core/crypto/lib/libsect283k1_ubuntu.so /usr/lib/ && \
    mkdir -p /etc/iotex/ && \
    echo "chain:" >> /etc/iotex/config.yaml && \
    echo "    producerPrivKey: \"925f0c9e4b6f6d92f2961d01aff6204c44d73c0b9d0da188582932d4fcad0d8ee8c66600\"" >> /etc/iotex/config.yaml && \
    echo "    producerPubKey: "336eb60a5741f585a8e81de64e071327a3b96c15af4af5723598a07b6121e8e813bbd0056ba71ae29c0d64252e913f60afaeb11059908b81ff27cbfa327fd371d35f5ec0cbc01705"" >> /etc/iotex/config.yaml && \
    echo "    genesisActionsPath: /etc/iotex/testnet_actions.yaml" >> /etc/iotex/config.yaml && \
    echo "network:" >> /etc/iotex/config.yaml && \
    echo "    bootstrapNodes:" >> /etc/iotex/config.yaml && \
    echo "        - \"127.0.0.1:4689\"" >> /etc/iotex/config.yaml && \
    echo "delegate:" >> /etc/iotex/config.yaml && \
    echo "    addrs:" >> /etc/iotex/config.yaml && \
    echo "        - \"127.0.0.1:4689\"" >> /etc/iotex/config.yaml && \
    ln -s $GOPATH/src/github.com/iotexproject/iotex-core/blockchain/testnet_actions.yaml /etc/iotex/testnet_actions.yaml

CMD [ "iotex-server", "-config-path=/etc/iotex/config.yaml"]
