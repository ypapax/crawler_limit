#!/usr/bin/env bash
set -ex

requestsPerSecond=$2
crawler_limit  -v 4 -log_dir /tmp -url $1 -n $requestsPerSecond