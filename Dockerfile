FROM golang

RUN \
  apt-get install -y --no-install-recommends make && \
  curl https://glide.sh/get | sh && \
  mkdir -p $GOPATH/src/github.com/iotexproject/ && \
  (cd $GOPATH/src/github.com/iotexproject/ && git clone https://github.com/iotexproject/iotex-core.git) && \
  (cd $GOPATH/src/github.com/iotexproject/iotex-core && glide i&& make build)

ENTRYPOINT ["/go/src/github.com/iotexproject/iotex-core/tools/start_node.sh"]

EXPOSE 40000-40005
