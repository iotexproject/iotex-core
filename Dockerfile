FROM golang:1.10.2-stretch

RUN apt-get install -y --no-install-recommends make && \
    mkdir -p $GOPATH/src/github.com/iotexproject/iotex-core/

COPY ./ $GOPATH/src/github.com/iotexproject/iotex-core/

RUN cd $GOPATH/src/github.com/iotexproject/iotex-core/ && \
    make clean build && \
    ln -s $GOPATH/src/github.com/iotexproject/iotex-core/bin/server /usr/local/bin/iotex-server  && \
    ln -s $GOPATH/src/github.com/iotexproject/iotex-core/bin/txinjector /usr/local/bin/iotex-txinjector && \
    mkdir -p /etc/iotex/ && \
    ln -s $GOPATH/src/github.com/iotexproject/iotex-core/config.yaml /etc/iotex/config.yaml && \
    mkdir -p /var/log/iotex/

CMD ["iotex-server", "-stderrthreshold=WARNING", "-log_dir=/var/log/iotex/server.log", "-config=/etc/iotex/config.yaml"]