#!/usr/bin/env bash
set -e

go build -o hugo-diff ./cmd/diff
go build -o hugo-difftool ./cmd/difftool
