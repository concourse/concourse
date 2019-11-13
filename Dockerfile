# NOTE: this Dockerfile is purely for local development! it is *not* used for
# the official 'concourse/concourse' image.

FROM concourse/dev


# download go modules separately so this doesn't re-run on every change
WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN grep '^replace' go.mod || go mod download


# containerd tooling
ARG RUNC_VERSION=v1.0.0-rc8
ARG CNI_VERSION=v0.8.2
ARG CONTAINERD_VERSION=1.3.0

RUN apt update && apt install -y curl

RUN set -x && \
	curl -sSL https://github.com/containerd/containerd/releases/download/v$CONTAINERD_VERSION/containerd-$CONTAINERD_VERSION.linux-amd64.tar.gz \
		| tar -zvxf - -C /usr/local/concourse/bin && \
	curl -sSL https://github.com/opencontainers/runc/releases/download/$RUNC_VERSION/runc.amd64 \ 
		-o /usr/local/concourse/bin/runc && chmod +x /usr/local/concourse/bin/runc && \
	curl -sSL https://github.com/containernetworking/plugins/releases/download/$CNI_VERSION/cni-plugins-linux-amd64-$CNI_VERSION.tgz \
		| tar -zvxf - -C /usr/local/concourse/bin


# build Concourse without using 'packr' and set up a volume so the web assets
# live-update
COPY . .
RUN go build -gcflags=all="-N -l" -o /usr/local/concourse/bin/concourse \
      ./cmd/concourse
VOLUME /src

# generate keys (with 1024 bits just so they generate faster)
RUN mkdir -p /concourse-keys
RUN concourse generate-key -t rsa -b 1024 -f /concourse-keys/session_signing_key
RUN concourse generate-key -t ssh -b 1024 -f /concourse-keys/tsa_host_key
RUN concourse generate-key -t ssh -b 1024 -f /concourse-keys/worker_key
RUN cp /concourse-keys/worker_key.pub /concourse-keys/authorized_worker_keys
