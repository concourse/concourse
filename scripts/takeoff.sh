#!/bin/sh

set -e

JOB_NAME=$1

if [ -z $JOB_NAME ]; then
  echo "usage: takeoff.sh job"
  exit 1
fi

if [ -z $ATC_URL ]; then
  echo "ATC_URL must be set"
  exit 1
fi

USERNAME=$(
  echo $ATC_URL |
  python -c "import sys, urlparse; print urlparse.urlparse(sys.stdin.read()).username"
)

PASSWORD=$(
  echo $ATC_URL |
  python -c "import sys, urlparse; print urlparse.urlparse(sys.stdin.read()).password"
)

HOST=$(
  echo $ATC_URL |
  python -c "import sys, urlparse; print urlparse.urlparse(sys.stdin.read()).hostname"
)

PORT=$(
  echo $ATC_URL |
  python -c "import sys, urlparse; print urlparse.urlparse(sys.stdin.read()).port"
)

curl \
  -s \
  -X POST \
  --user "${USERNAME}:${PASSWORD}" \
  "${HOST}:${PORT}/jobs/${JOB_NAME}/builds"
