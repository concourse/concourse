# NOTE: this Dockerfile is purely for local development! it is *not* used for
# the official 'concourse/concourse' image.

ARG base_image=concourse/dev
FROM ${base_image} AS base

# concourse/dev uses Wolfi. Refresh both the source certificates and the bundle
# to the release that distrusts the compromised eMudhra certificate chains.
RUN apk add --no-cache \
    'ca-certificates>=20260909-r3' \
    'ca-certificates-bundle>=20260909-r3'

ARG goproxy
ENV GOPROXY=$goproxy

# download go modules separately so this doesn't re-run on every change
WORKDIR /src
COPY go.mod .
COPY go.sum .
# don't do go mod download if there's a replace directive pointing to local filepath (./, ../)
RUN grep ' => (\.\/|\.\.\/)' go.mod || go mod download

# build the init executable for containerd
COPY ./cmd/init/init.c /tmp/init.c
RUN gcc -O2 -static -o /usr/local/concourse/bin/init /tmp/init.c && rm /tmp/init.c

# copy the rest separately so we don't constantly rebuild init
COPY . .

# build 'concourse' binary
RUN go build -gcflags=all="-N -l" -o /usr/local/concourse/bin/concourse \
      ./cmd/concourse


# separate build target to build the linux fly binary
FROM base AS with-fly
RUN go build -ldflags '-extldflags "-static"' -o /tmp/fly ./fly && \
      tar -C /tmp -czf /usr/local/concourse/fly-assets/fly-$(go env GOOS)-$(go env GOARCH).tgz fly && \
      rm /tmp/fly
VOLUME /src
ENV CONCOURSE_WEB_PUBLIC_DIR=/src/web/public


# extend base stage with setup for local docker-compose workflow
FROM base

# set up a volume so locally built web UI changes auto-propagate
VOLUME /src
ENV CONCOURSE_WEB_PUBLIC_DIR=/src/web/public
