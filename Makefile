MAKEFLAGS += --warn-undefined-variables
SHELL := /bin/bash -o pipefail
GIT_SHA=$(shell git rev-parse --short HEAD)
GIT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
GIT_TAG=$(shell git describe --tags --abbrev=0 --match='v[0-9]*.[0-9]*.[0-9]*' 2> /dev/null | sed 's/^.//')
BUILD_DATE=$(shell date)
VERSION_PKG="go.acuvity.ai/a3s/pkgs/version"
LDFLAGS ?= -ldflags="-w -s -X '$(VERSION_PKG).GitSha=$(GIT_SHA)' -X '$(VERSION_PKG).GitBranch=$(GIT_BRANCH)' -X '$(VERSION_PKG).GitTag=$(GIT_TAG)' -X '$(VERSION_PKG).BuildDate=$(BUILD_DATE)'"

export GO111MODULE = on

default: lint test build vuln sec

lint:
	golangci-lint run \
		--timeout=5m \
		--disable=govet  \
		--enable=errcheck \
		--enable=ineffassign \
		--enable=unused \
		--enable=unconvert \
		--enable=misspell \
		--enable=prealloc \
		--enable=nakedret \
		--enable=unparam \
		--enable=nilerr \
		--enable=bodyclose \
		--enable=errorlint \
		./...
test:
	go test ./... -vet off -race -cover -covermode=atomic -coverprofile=unit_coverage.out

sec:
	gosec -quiet ./...

vuln:
	govulncheck ./...

build:
	go build $(LDFLAGS) -trimpath .

remod:
	go get go.acuvity.ai/tg@master
	go get go.acuvity.ai/wsc@master
	go get go.acuvity.ai/regolithe@master
	go get go.acuvity.ai/bahamut@master
	go get go.acuvity.ai/elemental@master
	go get go.acuvity.ai/manipulate@master
	go get go.acuvity.ai/a3s@master
	go mod tidy
