MAKEFLAGS += --warn-undefined-variables
SHELL := /bin/bash -o pipefail

export GO111MODULE = on

default: lint test build

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
	gosec -quiet -exclude=G304 ./...

build:
	go build .

remod:
	go get go.acuvity.ai/tg@master
	go get go.acuvity.ai/wsc@master
	go get go.acuvity.ai/regolithe@master
	go get go.acuvity.ai/bahamut@master
	go get go.acuvity.ai/elemental@master
	go get go.acuvity.ai/manipulate@master
	go get go.acuvity.ai/a3s@master
	go mod tidy
