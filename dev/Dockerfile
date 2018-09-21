FROM golang:1 AS builder

# XXX: xfsprogs can be removed once we get rid of the grootfs initialization
# see https://github.com/concourse/concourse/issues/2574#issuecomment-423273310
RUN apt-get update && apt-get -y install \
      iproute2 \
      ca-certificates \
      file \
      btrfs-tools \
      xfsprogs \
      jq

# volume containing SSH keys for web <-> worker auth
VOLUME /concourse-keys

# volume for non-aufs/etc. mount for baggageclaim's driver
VOLUME /worker-state
ENV CONCOURSE_WORK_DIR /worker-state

# 'web' keys
ENV CONCOURSE_TSA_HOST_KEY        /concourse-keys/tsa_host_key
ENV CONCOURSE_TSA_AUTHORIZED_KEYS /concourse-keys/authorized_worker_keys
ENV CONCOURSE_SESSION_SIGNING_KEY /concourse-keys/session_signing_key

# 'worker' keys
ENV CONCOURSE_TSA_PUBLIC_KEY         /concourse-keys/tsa_host_key.pub
ENV CONCOURSE_TSA_WORKER_PRIVATE_KEY /concourse-keys/worker_key

# enable DNS proxy to support Docker's 127.x.x.x DNS server
ENV CONCOURSE_GARDEN_DNS_PROXY_ENABLE true

# workaround for unconditional grootfs xfs setup
RUN for i in $(seq 0 15); do \
      mknod -m 0660 /dev/loop$i b 7 $i; \
    done

# warm the module cache so we don't have to fetch every time
WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download

# fetch dependencies and resources
COPY dev ./dev
RUN ./dev/fetch-release-binary cloudfoundry/garden-runc-release gdn /usr/local/bin/gdn
RUN ./dev/fetch-release-binary Yelp/dumb-init 'dumb-init_.*_amd64$' /usr/local/bin/dumb-init
RUN ./dev/fetch-resources /concourse-resource-types

# point to fetched resource types
ENV CONCOURSE_RESOURCE_TYPES /concourse-resource-types

# build Concourse
COPY . .
RUN go build -o /usr/local/bin/concourse github.com/concourse/concourse/bin/cmd/concourse

ENTRYPOINT ["/usr/local/bin/dumb-init", "/usr/local/bin/concourse"]
