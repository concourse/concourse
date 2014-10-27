#!/bin/bash

echo -n "bump "
for submodule in $(git diff --cached --submodule | grep '^Submodule' | awk '{print $2}'); do
  echo -n "$(basename $submodule) "
done

echo
echo

if [ "$#" != "0" ]; then
  for id in "$@"; do
    echo "[finishes #${id}]"
  done

  echo
fi

git submodule status | awk '{print $2}' | xargs git diff --cached --submodule
