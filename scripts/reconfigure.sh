#!/usr/bin/env bash

set -e
set -o pipefail

check_installed() {
  if ! command -v $1 > /dev/null 2>&1; then
    printf "$1 must be installed before running this script!"
    exit 1
  fi
}

configure_pipeline() {
  local name=$1
  local url=$2
  local pipeline=$3

  printf "configuring the $name pipeline...\n"

  fly -t $url \
    configure \
    -c $pipeline \
    -vf <(lpass show "Shared-Concourse/Concourse Pipeline Credentials" --notes) \
    $name
}

check_installed lpass
check_installed fly

# Make sure we're up to date and that we're logged in.
lpass sync

username=$(lpass show Shared-Concourse/CI --username)
password=$(lpass show Shared-Concourse/CI --password)
url="https://$username:$password@ci.concourse.ci"

pipelines_path=$(realpath -L $(dirname $(realpath $0))/../ci/pipelines)

configure_pipeline main \
  $url \
  $pipelines_path/concourse.yml

configure_pipeline resources \
  $url \
  $pipelines_path/resources.yml

configure_pipeline images \
  $url \
  $pipelines_path/images.yml

