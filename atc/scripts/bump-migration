#!/bin/bash

set -e -u

if ! [ -x "$(command -v rename)" ]; then
  echo "Error: rename not installed. \`brew install rename\` if mac os." >&2
  exit 1
fi

migrations_dir=$(dirname $0)/../db/migration/migrations/

migrations_on_master=$(git ls-tree --name-only origin/master $migrations_dir)
migrations_on_branch=$(git ls-tree --name-only HEAD $migrations_dir)

new_migrations=$(comm -13 <(echo "$migrations_on_master") <(echo "$migrations_on_branch"))

echo $new_migrations | sort -n | xargs -n2 bash -c 'rename "s/[0-9]+_/$(date +%s)_/" "$@"; sleep 1' _
