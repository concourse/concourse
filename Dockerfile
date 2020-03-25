# syntax = docker/dockerfile:experimental

# NOTE: this Dockerfile is purely for local development! it is *not* used for
# the official 'concourse/concourse' image.
#
# Usage:
#
#  1. enable `buildkit` in dockerd (`/etc/docker/daemon.json` in linux or daemon
#  preferences in macOS):
#
#	{"experimental": true, "features": {"buildkit": true}}
#
#
#  2. build the image
#
#	with docker:
#	
#		docker build -t concourse/concourse:local .
#
#	with docker-compose
#
#		COMPOSE_DOCKER_CLI_BUILD=true docker-compose build
#

FROM concourse/dev

WORKDIR /src
COPY . .

# build concourse and containerd's init
RUN \
	--mount=type=cache,target=/root/.cache/go-build \
	--mount=type=cache,target=/go/pkg/mod \
	set -ex && \
		go build -gcflags=all="-N -l" \
			-o /usr/local/concourse/bin/concourse \
			./cmd/concourse && \
		gcc -O2 -static \
			-o /usr/local/concourse/bin/init \
			./cmd/init/init.c


# have a volume at runtime so that assets live-update
VOLUME /src


# generate keys (with 1024 bits just so they generate faster)
RUN set -ex && \
	mkdir -p /concourse-keys && \
	concourse generate-key -t rsa -b 1024 -f /concourse-keys/session_signing_key && \
	concourse generate-key -t ssh -b 1024 -f /concourse-keys/tsa_host_key && \
	concourse generate-key -t ssh -b 1024 -f /concourse-keys/worker_key && \
	cp /concourse-keys/worker_key.pub /concourse-keys/authorized_worker_keys
