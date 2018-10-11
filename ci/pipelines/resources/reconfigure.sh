#!/bin/bash

set -e -u

cd $(dirname $0)

for resource in $(cat resources); do
  echo "configuring $resource..."
  fly -t resources sp -p $resource -c <(jsonnet -V resource=$resource template.jsonnet)
  echo ""
done
