#!/bin/bash

set -e -u

for resource in $(cat resources); do
  echo "configuring $resource..."
  fly -t resources sp -p $resource -c <(jsonnet -V resource=$resource template.jsonnet)
  echo ""
done
