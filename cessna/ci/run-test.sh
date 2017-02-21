#!/bin/bash
# vim: set ft=sh

set -e

export GOPATH=${PWD}/concourse
export PATH=${PWD}/concourse/bin:$PATH


pushd concourse/src/github.com/concourse/atc/cessna
  mkdir buildroot-tar
  curl -o $HOME/buildroot.tar $ROOTFS_TAR_URL

  go install github.com/onsi/ginkgo/ginkgo
  export ROOTFS_TAR_PATH=$HOME/buildroot.tar
  ginkgo -r -nodes=3 -race -randomizeAllSpecs -randomizeSuites "$@"
popd
