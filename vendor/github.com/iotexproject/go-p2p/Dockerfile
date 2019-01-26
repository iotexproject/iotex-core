FROM golang:1.10.2-stretch

COPY ./ $GOPATH/src/github.com/zjshen14/go-p2p/

ARG SKIP_DEP=false

RUN if [ "$SKIP_DEP" != true ] ; \
    then \
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh && \
        cd $GOPATH/src/github.com/zjshen14/go-p2p && \
        	dep ensure -vendor-only; \
    fi

run cd $GOPATH/src/github.com/zjshen14/go-p2p && \
		go build -o ./bin/main -v ./main/main.go

CMD ["/go/src/github.com/zjshen14/go-p2p/bin/main"]
