# used for building concourse/dev. this shortens the dev feedback loop by:
#
# * baking in resource types
# * baking in dumb-init/gdn binaries
# * warming the module cache
# * warming the build cache

FROM golang:1

# XXX: xfsprogs can be removed once we get rid of the grootfs initialization
# see https://github.com/concourse/concourse/issues/2574#issuecomment-423273310
RUN apt-get update && apt-get -y install \
      iproute2 \
      ca-certificates \
      file \
      btrfs-tools \
      xfsprogs \
      jq

# workaround for unconditional grootfs xfs setup
RUN for i in $(seq 0 15); do \
      mknod -m 0660 /dev/loop$i b 7 $i; \
    done

# fetch dependencies
COPY dumb-init/dumb_init_*_amd64 /usr/local/bin/dumb-init
COPY garden-runc/gdn-* /usr/local/bin/gdn
RUN chmod +x /usr/local/bin/dumb-init && chmod +x /usr/local/bin/gdn

# extract resource types
ADD mock-resource/mock-resource-*.tgz /concourse-resource-types/mock
ADD registry-image-resource/registry-image-resource-*.tgz /concourse-resource-types/registry-image
ENV CONCOURSE_RESOURCE_TYPES /concourse-resource-types

# volume for non-aufs/etc. mount for baggageclaim's driver
VOLUME /worker-state
ENV CONCOURSE_WORK_DIR /worker-state

# enable DNS proxy to support Docker's 127.x.x.x DNS server
ENV CONCOURSE_GARDEN_DNS_PROXY_ENABLE true

# warm the module and build cache, but don't keep the source
COPY concourse /src
RUN cd /src && \
      go mod download && \
      go build -o /usr/local/bin/concourse github.com/concourse/concourse/bin/cmd/concourse && \
      rm -rf /src

ENTRYPOINT ["/usr/local/bin/dumb-init", "/usr/local/bin/concourse"]
