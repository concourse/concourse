#!/bin/bash

set -e -x

cd $(dirname $0)

builder=/tmp/base16-builder

if ! [ -d $builder ]; then
  git clone --depth 1 https://github.com/chriskempson/base16-builder $builder
fi

for scheme in ${builder}/schemes/*.yml; do
  ./gen-color.sh $scheme > ../../public/main.${name}.css
done
