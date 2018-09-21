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

# fetch dependencies
ADD https://github.com/Yelp/dumb-init/releases/download/v1.2.2/dumb-init_1.2.2_amd64          /usr/local/bin/dumb-init
ADD https://github.com/cloudfoundry/garden-runc-release/releases/download/v1.16.4/gdn-1.16.4  /usr/local/bin/gdn
RUN chmod +x /usr/local/bin/dumb-init && chmod +x /usr/local/bin/gdn

# fetch resources
ADD https://github.com/concourse/mirror-resource/releases/download/v0.0.1/mirror-resource-0.0.1.tgz  /tmp/
ADD https://github.com/concourse/registry-image-resource/releases/download/v0.1.0/registry-image-resource-0.1.0.tgz        /tmp/
ADD https://github.com/concourse/git-resource/releases/download/v1.0.0/git-resource-1.0.0.tgz        /tmp/
ADD https://github.com/concourse/s3-resource/releases/download/v1.0.0/s3-resource-1.0.0.tgz          /tmp/
RUN mkdir -p /concourse-resource-types/mirror && \
      tar -zxf /tmp/mirror-resource-*.tgz -C /concourse-resource-types/mirror && \
      mkdir -p /concourse-resource-types/registry-image && \
      tar -zxf /tmp/registry-image-resource-*.tgz -C /concourse-resource-types/registry-image && \
      mkdir -p /concourse-resource-types/git && \
      tar -zxf /tmp/git-resource-*.tgz -C /concourse-resource-types/git && \
      mkdir -p /concourse-resource-types/s3 && \
      tar -zxf /tmp/s3-resource-*.tgz -C /concourse-resource-types/s3 && \
      rm /tmp/*.tgz

# point to fetched resource types
ENV CONCOURSE_RESOURCE_TYPES /concourse-resource-types

# warm the module cache first so we don't have to fetch every time
WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download

# build Concourse
COPY . .
RUN go build -o /usr/local/bin/concourse github.com/concourse/concourse/bin/cmd/concourse

ENTRYPOINT ["/usr/local/bin/dumb-init", "/usr/local/bin/concourse"]
