#!/usr/bin/env bash
set -ex

run(){
    go get ./...
    go install
    crawler_limit -alsologtostderr -v 4 $@
}

$@