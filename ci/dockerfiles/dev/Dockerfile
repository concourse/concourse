# used for building concourse/dev. this shortens the dev feedback loop by:
#
# * baking in resource types
# * baking in dumb-init/gdn binaries
# * warming the module cache
# * warming the build cache

FROM golang:1

RUN apt-get update && apt-get -y install \
      iproute2 \
      ca-certificates \
      file \
      btrfs-tools \
      jq

# fetch dependencies
COPY dumb-init/dumb-init_*_amd64 /usr/local/bin/dumb-init
COPY gdn/gdn-* /usr/local/bin/gdn
RUN chmod +x /usr/local/bin/dumb-init && chmod +x /usr/local/bin/gdn

# extract resource types
COPY mock-resource/mock-resource-*.tgz /concourse-resource-types/mock.tgz
COPY registry-image-resource/registry-image-resource-*.tgz /concourse-resource-types/registry-image.tgz
COPY time-resource/time-resource-*.tgz /concourse-resource-types/time.tgz
COPY git-resource/git-resource-*.tgz /concourse-resource-types/git.tgz
RUN set -e; for t in mock registry-image time git; do \
      mkdir /concourse-resource-types/$t; \
      tar -C /concourse-resource-types/$t -zxf /concourse-resource-types/$t.tgz; \
      rm /concourse-resource-types/$t.tgz; \
    done
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
      go install github.com/gobuffalo/packr/packr && \
      packr build -gcflags=all="-N -l" -o /usr/local/bin/concourse github.com/concourse/concourse/bin/cmd/concourse && \
      rm -rf /src

ENTRYPOINT ["/usr/local/bin/dumb-init", "/usr/local/bin/concourse"]
