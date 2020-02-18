#!/bin/bash

set -e -u

cd $(dirname $0)/..

container_name=""
dlv_flags=""
docker_flags="--interactive --privileged --rm --tty"

usage() {
    echo "Usage: trace (web|worker) [--listen port]"
    exit 1
}

while test $# -gt 0; do
   case "$1" in
        web)
            container_name="concourse_web_1"
            shift
            ;;
        worker)
            container_name="concourse_worker_1"
            shift
            ;;
        --listen)
            shift
            dlv_flags=" --headless=true --listen=:$1"
            docker_flags+=" -p $1:$1"
            shift
            ;;
        *)
            usage
            ;;
  esac
done

if [ -z "$container_name" ]; then
  usage
fi

trace_pid=$(docker exec $container_name pidof concourse)

docker build --tag dlv ./hack/dlv

docker run $docker_flags \
  --pid=container:$container_name \
  dlv attach $trace_pid $dlv_flags
