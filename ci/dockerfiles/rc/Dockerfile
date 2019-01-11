FROM ubuntu:bionic


ENV \
  CONCOURSE_RESOURCE_TYPES=/concourse-resource-types \
  CONCOURSE_WORK_DIR=/worker-state \
  CONCOURSE_GARDEN_DNS_PROXY_ENABLE=true


RUN apt update && apt install -y \
    ca-certificates \
    btrfs-tools \
    dumb-init \
    iproute2


ADD ./rc/bin/concourse /usr/local/bin
ADD ./rc/bin/gdn /usr/local/bin
ADD ./rc/resource-types /concourse-resource-types
ADD ./rc/fly-assets /


ENTRYPOINT ["dumb-init", "/usr/local/bin/concourse"]

