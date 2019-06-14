#!/usr/bin/env bash
set -ex

run(){
    go get ./...
    go install
    crawler_limit  -v 4 -log_dir /tmp $@
}

rund(){
	docker build -t test-crawler . && docker run test-crawler $@
}
$@