#!/bin/bash

set -e -x

cd $(dirname $0)

builder=/tmp/base16-builder

if ! [ -d $builder ]; then
  git clone --depth 1 https://github.com/concourse/base16-builder $builder
fi

for scheme in ${builder}/schemes/*.yml; do
  name=$(echo $(basename "$scheme") | sed -e 's/.yml//')
  ./gen-color.sh $scheme > ../../public/main.${name}.css
done
