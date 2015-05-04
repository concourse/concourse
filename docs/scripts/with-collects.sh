#!/bin/sh

cd $(dirname $0)/..

MAIN_COLLECTS=$(racket -e '
  (let ([main-paths (map path->string (current-library-collection-paths))])
    (displayln (string-join main-paths ":")))
')

export PLTCOLLECTS=$MAIN_COLLECTS:${PWD}/collects

"$@"
