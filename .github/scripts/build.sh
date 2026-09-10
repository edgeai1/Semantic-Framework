#!/usr/bin/env bash
# Copyright 2026 InsightOS
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail
# Standard tests use the system compiler; production binaries use musl.
go test -p 2 ./... -race -count=1 -timeout=10m
mkdir -p .output/bin
version="${TARGET_TAG#v}"
version="${version:-0.0.0-ci}"
for name in semantic-server semantic-pilot semantic; do
  CGO_ENABLED=1 CC=musl-gcc go build -p 2 -trimpath -tags musl,netgo,osusergo \
    -ldflags "-X insightos.cn/semantic-framework/pkg/version.Version=$version -linkmode external -extldflags '-static'" \
    -o ".output/bin/$name" "./cmd/$name"
done
.output/bin/semantic-server --help
.output/bin/semantic-pilot --help
# The CLI uses exit 2 for help in this source baseline.
status=0
.output/bin/semantic --help || status=$?
[[ "$status" == 0 || "$status" == 2 ]]
