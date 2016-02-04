#!/bin/bash

scheme="$1"

name=$(echo $(basename "$scheme") | sed -e 's/.yml//')

grep '^base[0-9A-F]\{2\}' "$scheme" | \
  sed -e 's/^base\([0-9A-F]\{2\}\): "\(.*\)".*/@base\1: #\2;/' > \
  _vars.less

echo "@theme: \"$name\";" >> _vars.less

lessc main.less
