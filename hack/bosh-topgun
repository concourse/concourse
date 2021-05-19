#!/bin/bash

set -e -u

cd $(dirname $0)/..

if [ "$#" != "2" ]; then
  echo "usage: $0 <concourse-bosh-release> <ci>" >&2
  exit 1
fi

release="$1"
ci="$2"
concourse="$(pwd)"

pushd "$release"
  ./dev/build-concourse "$concourse"

  version="0.0.0-topgun.$(date +%s)"

  release_input=/tmp/hack-topgun/$version
  mkdir -p $release_input
  echo $version > ${release_input}/version
  bosh create-release --version $version --tarball ${release_input}/dev.tgz
popd

export BOSH_ENVIRONMENT=https://10.0.0.6:25555
export BOSH_CA_CERT="((testing_bosh_ca_cert))"
export BOSH_CLIENT="((testing_bosh_client.id))"
export BOSH_CLIENT_SECRET="((testing_bosh_client.secret))"

fly -t ci execute \
  -c "$ci/tasks/topgun.yml" \
  -j concourse/bosh-topgun-runtime \
  -i concourse=. \
  -i ci="$ci" \
  -i concourse-release=${release_input} \
  -m stemcell=gcp-bionic-stemcell \
  --tag bosh \
  -- \
  --stream \
  "$@"
