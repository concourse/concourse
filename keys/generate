#!/usr/bin/env bash

set -e -u

cd $(dirname $0)

docker run --rm -v $PWD/web:/keys concourse/concourse \
  generate-key -t rsa -f /keys/session_signing_key

docker run --rm -v $PWD/web:/keys concourse/concourse \
  generate-key -t ssh -f /keys/tsa_host_key

docker run --rm -v $PWD/worker:/keys concourse/concourse \
  generate-key -t ssh -f /keys/worker_key

cp ./worker/worker_key.pub ./web/authorized_worker_keys
cp ./web/tsa_host_key.pub ./worker
