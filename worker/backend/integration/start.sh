#!/usr/bin/env bash

set -m

echo "starting runc"
runc &

echo "starting containerd"
containerd &

/bin/bash
