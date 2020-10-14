# NOTE: this Dockerfile is purely for local development! it is *not* used for
# the official 'concourse/concourse' image.

FROM concourse/dev AS base

# generate keys (with 1024 bits just so they generate faster)
RUN mkdir -p /concourse-keys
RUN concourse generate-key -t rsa -b 1024 -f /concourse-keys/session_signing_key
RUN concourse generate-key -t ssh -b 1024 -f /concourse-keys/tsa_host_key
RUN concourse generate-key -t ssh -b 1024 -f /concourse-keys/worker_key
RUN cp /concourse-keys/worker_key.pub /concourse-keys/authorized_worker_keys

# 'web' keys
ENV CONCOURSE_SESSION_SIGNING_KEY     /concourse-keys/session_signing_key
ENV CONCOURSE_TSA_AUTHORIZED_KEYS     /concourse-keys/authorized_worker_keys
ENV CONCOURSE_TSA_HOST_KEY            /concourse-keys/tsa_host_key

# 'worker' keys
ENV CONCOURSE_TSA_PUBLIC_KEY          /concourse-keys/tsa_host_key.pub
ENV CONCOURSE_TSA_WORKER_PRIVATE_KEY  /concourse-keys/worker_key

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
      go build --tags 'netgo osusergo' -a -ldflags '-extldflags "-static"' -o /tmp/fly ./fly && \
      tar -C /tmp -czf /usr/local/concourse/fly-assets/fly-$(go env GOOS)-$(go env GOARCH).tgz fly && \
      rm /tmp/fly

# extend base stage with setup for local docker-compose workflow
FROM base

# set up a volume so locally built web UI changes auto-propagate
VOLUME /src
