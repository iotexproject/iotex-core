FROM golang:1.21-bullseye as build

WORKDIR apps/iotex-core

RUN apt-get update && apt-get install -y make gcc musl-dev git libc-dev build-essential linux-headers-amd64 ca-certificates

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .
ARG PACKAGE_VERSION
ARG PACKAGE_COMMIT_ID
ARG GIT_STATUS
RUN mkdir -p $GOPATH/pkg/linux_amd64/github.com/iotexproject/ && \
    PACKAGE_VERSION=$PACKAGE_VERSION PACKAGE_COMMIT_ID=$PACKAGE_COMMIT_ID GIT_STATUS=$GIT_STATUS make clean build-all

FROM golang:1.21-bullseye

RUN mkdir -p /etc/iotex/
COPY --from=build /go/apps/iotex-core/bin/server /usr/local/bin/iotex-server
COPY --from=build /go/apps/iotex-core/bin/actioninjectorv2 /usr/local/bin/iotex-actioninjectorv2
COPY --from=build /go/apps/iotex-core/bin/addrgen /usr/local/bin/iotex-addrgen
COPY --from=build /go/apps/iotex-core/bin/ioctl /usr/local/bin/ioctl


# logrotate log file daily
RUN apt-get update && apt-get install -y logrotate
COPY logrotate.conf /etc/logrotate.d/iotex
RUN mkdir -p /var/lib/
RUN touch /var/lib/logrotate.status

# Create a cron job for logrotate
RUN echo -e "#!/bin/sh\n\n/usr/sbin/logrotate -f /etc/logrotate.d/iotex" > /etc/cron.daily/logrotate
RUN chmod +x /etc/cron.daily/logrotate

COPY entrypoint.sh /usr/local/bin
RUN chmod +x /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["iotex-server"]
