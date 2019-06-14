#!/usr/bin/env bash
set -ex

requestsPerSecond=$2
if [ -z "$requestsPerSecond" ]; then
	requestsPerSecond=1
fi
crawler_limit  -v 4 -log_dir /tmp -url $1 -n $requestsPerSecond