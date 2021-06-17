#!/bin/bash

trap 'restart=1' HUP

while true; do
  /usr/local/concourse/bin/concourse worker & pid="$!"

  restart=0

  while [ "$restart" -ne 1 ]; do sleep 1; done
  rm -f /ready
  kill -TERM "$pid"
  # wait for the existing process to exit
  while ps -p $pid > /dev/null; do
    sleep 1
    echo "concourse is still running at pid $pid"
  done
  touch /ready
done
