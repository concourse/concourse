#!/bin/bash

set -e -x

cd $(dirname $0)

builder=/tmp/base16-builder

if ! [ -d $builder ]; then
  git clone --depth 1 https://github.com/chriskempson/base16-builder $builder
fi

for scheme in ${builder}/schemes/*.yml; do
  name=$(echo $(basename $scheme) | sed -e 's/.yml//')

  grep '^base[0-9A-F]\{2\}' $scheme | \
    sed -e 's/^base\([0-9A-F]\{2\}\): "\(.*\)".*/@base\1: #\2;/' > \
    _vars.less

  echo "@theme: \"$name\";" >> _vars.less

  lessc main.less ../../public/main.${name}.css
done
