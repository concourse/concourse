#!/bin/sh

set -e

JOB_NAME=$1

if [ -z "$JOB_NAME" ]; then
  echo "usage: takeoff.sh job"
  exit 1
fi

if [ -z "$ATC_URL" ]; then
  echo "ATC_URL must be set"
  exit 1
fi

read USERNAME PASSWORD HOST PORT < <(
  echo -n "$ATC_URL" | python -c "
import sys, urlparse
url = urlparse.urlparse(sys.stdin.read())
port = (url.port or (url.scheme == 'https' and 443 or 80))
print url.username, url.password, url.hostname, port
"
)

curl \
  -s \
  -X POST \
  --user "${USERNAME}:${PASSWORD}" \
  "${HOST}:${PORT}/jobs/${JOB_NAME}/builds"
