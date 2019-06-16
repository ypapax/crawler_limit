#!/usr/bin/env bash
set -ex

run(){
    go get ./...
    go install
    crawler_limit  -v 4 -log_dir /tmp -url $@
}

runrace(){
    go get ./...
    CGO_ENABLED=1 go install -race
    crawler_limit  -v 4 -log_dir /tmp -url $@
}

rund(){
	docker build -t test-crawler . && docker run test-crawler $@
}
$@