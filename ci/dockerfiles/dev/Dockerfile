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
COPY bosh-io-release-resource/bosh-io-release-resource-*.tgz /concourse-resource-types/bosh-io-release.tgz
COPY bosh-io-stemcell-resource/bosh-io-stemcell-resource-*.tgz /concourse-resource-types/bosh-io-stemcell.tgz
COPY cf-resource/cf-resource-*.tgz /concourse-resource-types/cf.tgz
COPY docker-image-resource/docker-image-resource-*.tgz /concourse-resource-types/docker-image.tgz
COPY git-resource/git-resource-*.tgz /concourse-resource-types/git.tgz
COPY github-release-resource/github-release-resource-*.tgz /concourse-resource-types/github-release.tgz
COPY hg-resource/hg-resource-*.tgz /concourse-resource-types/hg.tgz
COPY pool-resource/pool-resource-*.tgz /concourse-resource-types/pool.tgz
COPY registry-image-resource/registry-image-resource-*.tgz /concourse-resource-types/registry-image.tgz
COPY s3-resource/s3-resource-*.tgz /concourse-resource-types/s3.tgz
COPY semver-resource/semver-resource-*.tgz /concourse-resource-types/semver.tgz
COPY time-resource/time-resource-*.tgz /concourse-resource-types/time.tgz
COPY tracker-resource/tracker-resource-*.tgz /concourse-resource-types/tracker.tgz
COPY mock-resource/mock-resource-*.tgz /concourse-resource-types/mock.tgz
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
