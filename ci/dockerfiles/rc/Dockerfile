FROM ubuntu:bionic AS ubuntu

FROM ubuntu AS assets
COPY ./rc/*.tgz /tmp
RUN tar xzf /tmp/*tgz -C /usr/local


FROM ubuntu

# 'web' keys
ENV CONCOURSE_SESSION_SIGNING_KEY     /concourse-keys/session_signing_key
ENV CONCOURSE_TSA_AUTHORIZED_KEYS     /concourse-keys/authorized_worker_keys
ENV CONCOURSE_TSA_HOST_KEY            /concourse-keys/tsa_host_key

# 'worker' keys
ENV CONCOURSE_TSA_PUBLIC_KEY          /concourse-keys/tsa_host_key.pub
ENV CONCOURSE_TSA_WORKER_PRIVATE_KEY  /concourse-keys/worker_key

# enable DNS proxy to support Docker's 127.x.x.x DNS server
ENV CONCOURSE_GARDEN_DNS_PROXY_ENABLE         true
ENV CONCOURSE_WORKER_GARDEN_DNS_PROXY_ENABLE  true

# auto-wire work dir for 'worker' and 'quickstart'
ENV CONCOURSE_WORK_DIR                /worker-state
ENV CONCOURSE_WORKER_WORK_DIR         /worker-state

# location for base resource types
ENV CONCOURSE_RESOURCE_TYPES          /usr/local/concourse/resource-types

# volume for non-aufs/etc. mount for baggageclaim's driver
VOLUME /worker-state

RUN apt update && apt install -y \
    btrfs-tools \
    ca-certificates \
    dumb-init \
    iproute2

COPY --from=assets /usr/local/concourse /usr/local/concourse


ENTRYPOINT ["dumb-init", "/usr/local/concourse/bin/concourse"]
