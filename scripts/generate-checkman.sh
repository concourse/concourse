#!/usr/bin/env bash

set -e
set -o pipefail

pipelines() {
  fly -t ci pipelines |
    grep -v acceptance |
    grep -v atomy |
    awk '{print $1}'
}

mkdir -p ~/Checkman

pushd ~/Checkman
  pipelines | while read pipeline; do
    echo -n "saving $pipeline checks... "
    fly -t ci cl -p $pipeline > $pipeline
    echo "done"
  done
popd

