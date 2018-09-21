FROM ubuntu:18.04

# XXX: xfsprogs can be removed once we get rid of the grootfs initialization
# see https://github.com/concourse/concourse/issues/2574#issuecomment-423273310
RUN apt-get update && apt-get -y install \
      iproute2 \
      ca-certificates \
      file \
      btrfs-tools \
      xfsprogs

# volume for non-aufs/etc. mount for baggageclaim's driver
VOLUME /worker-state
ENV CONCOURSE_WORK_DIR /worker-state
ENV CONCOURSE_WORKER_WORK_DIR /worker-state

# volume containing resources to use for workers
VOLUME /concourse-resource-types
ENV CONCOURSE_RESOURCE_TYPES /concourse-resource-types
ENV CONCOURSE_WORKER_RESOURCE_TYPES /concourse-resource-types


# volume containing SSH keys for web <-> worker auth
VOLUME /concourse-keys

# 'web' keys
ENV CONCOURSE_TSA_HOST_KEY        /concourse-keys/tsa_host_key
ENV CONCOURSE_TSA_AUTHORIZED_KEYS /concourse-keys/authorized_worker_keys
ENV CONCOURSE_SESSION_SIGNING_KEY /concourse-keys/session_signing_key

# 'worker' keys
ENV CONCOURSE_TSA_PUBLIC_KEY         /concourse-keys/tsa_host_key.pub
ENV CONCOURSE_TSA_WORKER_PRIVATE_KEY /concourse-keys/worker_key

# enable DNS proxy to support Docker's 127.x.x.x DNS server
ENV CONCOURSE_GARDEN_DNS_PROXY_ENABLE true
ENV CONCOURSE_WORKER_GARDEN_DNS_PROXY_ENABLE true

# workaround for unconditional grootfs xfs setup
RUN for i in $(seq 0 15); do \
      mknod -m 0660 /dev/loop$i b 7 $i; \
    done

COPY gdn /usr/local/bin/gdn
COPY dumb-init /usr/local/bin/dumb-init
COPY concourse /usr/local/bin/concourse

ENTRYPOINT ["/usr/local/bin/dumb-init", "/usr/local/bin/concourse"]
