# Copyright 2020 Changkun Ou. All rights reserved.
# Use of this source code is governed by a GPL-3.0
# license that can be found in the LICENSE file.

VERSION = $(shell git describe --always --tags)
BUILDTIME = $(shell date +%FT%T%z)
GOPATH=$(shell go env GOPATH)
IMAGE = midgard
BINARY = mg
TARGET = -o $(BINARY)
MIDGARD_HOME = changkun.de/x/midgard
BUILD_SETTINGS = -ldflags="-X $(MIDGARD_HOME)/pkg/version.GitVersion=$(VERSION) -X $(MIDGARD_HOME)/pkg/version.BuildTime=$(BUILDTIME)"
BUILD_FLAGS = $(TARGET) $(BUILD_SETTINGS) -mod=vendor

all:
	go build $(BUILD_FLAGS)
install:
	go get google.golang.org/protobuf/cmd/protoc-gen-go \
         google.golang.org/grpc/cmd/protoc-gen-go-grpc
gen:
	go generate ./...
build:
	go generate ./...
	GOOS=linux go build $(BUILD_FLAGS)
	docker build -t $(IMAGE):$(VERSION) -t $(IMAGE):latest -f docker/Dockerfile .
up: down
	docker-compose -f docker/compose.yml up -d
down:
	docker-compose -f docker/compose.yml down
clean: down
	rm -rf $(BINARY)
	docker rmi -f $(shell docker images -f "dangling=true" -q) 2> /dev/null; true
	docker rmi -f $(IMAGE):latest $(IMAGE):$(VERSION) 2> /dev/null; true
