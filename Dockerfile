# NOTE: this Dockerfile is purely for local development! it is *not* used for
# the official 'concourse/concourse' image.

ARG base_image=concourse/dev
FROM ${base_image} AS base

# download go modules separately so this doesn't re-run on every change
WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN grep '^replace' go.mod || go mod download

# build the init executable for containerd
COPY ./cmd/init/init.c /tmp/init.c
RUN gcc -O2 -static -o /usr/local/concourse/bin/init /tmp/init.c && rm /tmp/init.c

# build Concourse without using 'packr' so that the volume in the next stage
# can live-update
COPY . .
RUN go build -gcflags=all="-N -l" -o /usr/local/concourse/bin/concourse \
      ./cmd/concourse
RUN set -x && \
      go build -ldflags '-extldflags "-static"' -o /tmp/fly ./fly && \
      tar -C /tmp -czf /usr/local/concourse/fly-assets/fly-$(go env GOOS)-$(go env GOARCH).tgz fly && \
      rm /tmp/fly

# extend base stage with setup for local docker-compose workflow
FROM base

# set up a volume so locally built web UI changes auto-propagate
VOLUME /src
