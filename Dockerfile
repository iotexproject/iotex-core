FROM golang:1.11.5-stretch

WORKDIR $GOPATH/src/github.com/iotexproject/iotex-core/

RUN apt-get install -y --no-install-recommends make

COPY . .

RUN mkdir -p $GOPATH/pkg/linux_amd64/github.com/iotexproject/ && \
    make clean build && \
    cp $GOPATH/src/github.com/iotexproject/iotex-core/bin/server /usr/local/bin/iotex-server  && \
    cp $GOPATH/src/github.com/iotexproject/iotex-core/bin/actioninjectorv2 /usr/local/bin/iotex-actioninjectorv2 && \
    cp $GOPATH/src/github.com/iotexproject/iotex-core/bin/addrgen /usr/local/bin/iotex-addrgen && \
    cp $GOPATH/src/github.com/iotexproject/iotex-core/bin/ioctl /usr/local/bin/ioctl && \
    cp ./crypto/lib/libsect283k1_ubuntu.so /usr/lib/ && \
    mkdir -p /etc/iotex/ && \
    rm -rf $GOPATH/src/github.com/iotexproject/iotex-core/

CMD [ "iotex-server"]
