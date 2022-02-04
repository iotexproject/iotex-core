FROM golang:1.17-alpine as build

WORKDIR apps/iotex-core

RUN apk add --no-cache make gcc musl-dev linux-headers git ca-certificates

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN mkdir -p $GOPATH/pkg/linux_amd64/github.com/iotexproject/ && \
    make clean build-all

FROM alpine:latest

RUN mkdir -p /etc/iotex/
COPY --from=build /go/apps/iotex-core/bin/server /usr/local/bin/iotex-server
COPY --from=build /go/apps/iotex-core/bin/actioninjectorv2 /usr/local/bin/iotex-actioninjectorv2
COPY --from=build /go/apps/iotex-core/bin/addrgen /usr/local/bin/iotex-addrgen
COPY --from=build /go/apps/iotex-core/bin/ioctl /usr/local/bin/ioctl

CMD [ "iotex-server"]

# logrotate log file daily
RUN apk add logrotate
COPY logrotate.conf /etc/logrotate.d/iotex
RUN mkdir -p /var/lib/
RUN touch /var/lib/logrotate.status
RUN logrotate /etc/logrotate.d/iotex
