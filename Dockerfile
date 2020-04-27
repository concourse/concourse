# NOTE: this Dockerfile is purely for local development! it is *not* used for
# the official 'concourse/concourse' image.

FROM concourse/dev


# download go modules separately so this doesn't re-run on every change
WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN grep '^replace' go.mod || go mod download

# build Concourse without using 'packr' and set up a volume so the web assets
# live-update
COPY . .
RUN go build -gcflags=all="-N -l" -o /usr/local/concourse/bin/concourse \
      ./cmd/concourse
VOLUME /src


# build the init executable for containerd
RUN  set -x && \
	gcc -O2 -static -o /usr/local/concourse/bin/init ./cmd/init/init.c


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
