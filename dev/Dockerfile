FROM concourse/mirror-resource AS mirror

FROM golang:1

# XXX: xfsprogs can be removed once we get rid of the grootfs initialization
# see https://github.com/concourse/concourse/issues/2574#issuecomment-423273310
RUN apt-get update && apt-get -y install \
      iproute2 \
      ca-certificates \
      file \
      btrfs-tools \
      xfsprogs \
      jq

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

# volume for non-aufs/etc. mount for baggageclaim's driver
VOLUME /worker-state
ENV CONCOURSE_WORK_DIR /worker-state

# enable DNS proxy to support Docker's 127.x.x.x DNS server
ENV CONCOURSE_GARDEN_DNS_PROXY_ENABLE true

# workaround for unconditional grootfs xfs setup
RUN for i in $(seq 0 15); do \
      mknod -m 0660 /dev/loop$i b 7 $i; \
    done

# fetch dependencies
ADD https://github.com/Yelp/dumb-init/releases/download/v1.2.2/dumb-init_1.2.2_amd64          /usr/local/bin/dumb-init
ADD https://github.com/cloudfoundry/garden-runc-release/releases/download/v1.16.4/gdn-1.16.4  /usr/local/bin/gdn
RUN chmod +x /usr/local/bin/dumb-init && chmod +x /usr/local/bin/gdn

# warm the module cache first so we don't have to fetch every time
WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download

# fetch resources
COPY --from=mirror / /concourse-resource-types/mirror/rootfs
RUN echo '{"type":"mirror","version":"'$(date +%s)'"}' > /concourse-resource-types/mirror/resource_metadata.json

# point to fetched resource types
ENV CONCOURSE_RESOURCE_TYPES /concourse-resource-types

# build Concourse
COPY . .
RUN go build -o /usr/local/bin/concourse github.com/concourse/concourse/bin/cmd/concourse

ENTRYPOINT ["/usr/local/bin/dumb-init", "/usr/local/bin/concourse"]
