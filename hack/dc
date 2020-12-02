#!/bin/bash

set -e -u

cd $(dirname $0)/..

args=(-f docker-compose.yml)
core=(vault prometheus jaeger)

for f in "${core[@]}"; do
  args+=(-f "hack/overrides/$f.yml")
done

docker-compose ${args[@]} "$@"
