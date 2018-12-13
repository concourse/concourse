# NOTE: this Dockerfile is purely for local development! it is *not* used for
# the official 'concourse/concourse' image.

FROM concourse/dev

# generate one-off keys for development
RUN mkdir /concourse-keys
RUN ssh-keygen -t rsa -N '' -f /concourse-keys/tsa_host_key
RUN ssh-keygen -t rsa -N '' -f /concourse-keys/session_signing_key
RUN ssh-keygen -t rsa -N '' -f /concourse-keys/worker_key
RUN cp /concourse-keys/worker_key.pub /concourse-keys/authorized_worker_keys

# keys for 'web'
ENV CONCOURSE_TSA_HOST_KEY        /concourse-keys/tsa_host_key
ENV CONCOURSE_TSA_AUTHORIZED_KEYS /concourse-keys/authorized_worker_keys
ENV CONCOURSE_SESSION_SIGNING_KEY /concourse-keys/session_signing_key

# keys for 'worker'
ENV CONCOURSE_TSA_PUBLIC_KEY         /concourse-keys/tsa_host_key.pub
ENV CONCOURSE_TSA_WORKER_PRIVATE_KEY /concourse-keys/worker_key

RUN mkdir /src
WORKDIR /src

# download go modules separately to enable caching
COPY go.mod .
COPY go.sum .
RUN go mod download

# download yarn packages separately to enable caching
COPY package.json .
COPY yarn.lock .
RUN yarn install

# build web UI separately so we only build if necessary
COPY web web
RUN yarn build

# build Concourse
COPY . .
RUN go build -gcflags=all="-N -l" -o /usr/local/bin/concourse github.com/concourse/concourse/bin/cmd/concourse

# override /src with a volume so we get live-updated packr stuff
VOLUME /src
