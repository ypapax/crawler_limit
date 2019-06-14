#!/usr/bin/env bash
set -ex

crawler_limit  -v 4 -log_dir /tmp -url $@