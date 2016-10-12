#!/bin/bash

set -e

not_installed() {
  ! command -v $1 > /dev/null 2>&1
}

atc_dir=$(cd $(dirname $0)/.. && pwd)

if not_installed ginkgo; then
  echo "# ginkgo is not installed! run the following command:"
  echo "    go install github.com/onsi/ginkgo/ginkgo"
  exit 1
fi

if not_installed npm; then
  echo "# npm is not installed! run the following command:"
  echo "    brew install node"
  exit 1
fi

cd $atc_dir
ginkgo -r -p -race "$@"
