# used for building concourse/dev. this shortens the dev feedback loop by:
#
# * baking in the gdn binary
# * baking in fly binaries
# * baking in resource types
# * warming the module cache
# * warming the build cache

FROM golang:1

RUN apt-get update && apt-get -y install \
      iproute2 \
      ca-certificates \
      file \
      btrfs-tools \
      dumb-init

# add and install executable dependencies
COPY gdn/gdn-* /usr/local/concourse/bin/gdn
RUN chmod +x /usr/local/concourse/bin/gdn

# add fly executables
RUN mkdir /usr/local/concourse/fly-assets
COPY fly-linux/fly-*.tgz /usr/local/concourse/fly-assets
COPY fly-windows/fly-*.zip /usr/local/concourse/fly-assets
COPY fly-darwin/fly-*.tgz /usr/local/concourse/fly-assets

# add and extract resource types
COPY bosh-io-release-resource/bosh-io-release-resource-*.tgz    /concourse-resource-types/bosh-io-release.tgz
COPY bosh-io-stemcell-resource/bosh-io-stemcell-resource-*.tgz  /concourse-resource-types/bosh-io-stemcell.tgz
COPY cf-resource/cf-resource-*.tgz                              /concourse-resource-types/cf.tgz
COPY docker-image-resource/docker-image-resource-*.tgz          /concourse-resource-types/docker-image.tgz
COPY git-resource/git-resource-*.tgz                            /concourse-resource-types/git.tgz
COPY github-release-resource/github-release-resource-*.tgz      /concourse-resource-types/github-release.tgz
COPY hg-resource/hg-resource-*.tgz                              /concourse-resource-types/hg.tgz
COPY mock-resource/mock-resource-*.tgz                          /concourse-resource-types/mock.tgz
COPY pool-resource/pool-resource-*.tgz                          /concourse-resource-types/pool.tgz
COPY registry-image-resource/registry-image-resource-*.tgz      /concourse-resource-types/registry-image.tgz
COPY s3-resource/s3-resource-*.tgz                              /concourse-resource-types/s3.tgz
COPY semver-resource/semver-resource-*.tgz                      /concourse-resource-types/semver.tgz
COPY time-resource/time-resource-*.tgz                          /concourse-resource-types/time.tgz
COPY tracker-resource/tracker-resource-*.tgz                    /concourse-resource-types/tracker.tgz
RUN set -e; \
      for tgz in /concourse-resource-types/*.tgz; do \
        file=$(basename $tgz); \
        dir=/usr/local/concourse/resource-types/${file%.*}; \
        mkdir -p $dir; \
        tar -C $dir -zxf $tgz; \
      done; \
      rm -rf /concourse-resource-types

# install concourse, leaving the module cache populated
#
# nuke /src after to ensure dev builds build from a clean slate (otherwise this
# can cause build failures if e.g. files are removed locally)
COPY concourse /src
RUN cd /src && \
      go mod download && \
      go install github.com/gobuffalo/packr/packr && \
      packr build -gcflags=all="-N -l" -o /usr/local/concourse/bin/concourse \
        ./cmd/concourse && \
      rm -rf /src

# volume for non-aufs/etc. mount for baggageclaim's driver
VOLUME /worker-state
ENV CONCOURSE_WORK_DIR /worker-state

# enable DNS proxy to support Docker's 127.x.x.x DNS server
ENV CONCOURSE_GARDEN_DNS_PROXY_ENABLE true

# set $PATH for convenience
ENV PATH /usr/local/concourse/bin:${PATH}

ENTRYPOINT ["dumb-init", "concourse"]
