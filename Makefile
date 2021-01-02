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
BUILD_SETTINGS = -ldflags="-X $(MIDGARD_HOME)/internal/version.GitVersion=$(VERSION) -X $(MIDGARD_HOME)/internal/version.BuildTime=$(BUILDTIME)"
BUILD_FLAGS = $(BUILD_SETTINGS) -mod=vendor

all:
	go build $(TARGET) $(BUILD_FLAGS)
install:
	go get google.golang.org/protobuf/cmd/protoc-gen-go \
         google.golang.org/grpc/cmd/protoc-gen-go-grpc
gen:
	go generate ./...
dep:
	go mod tidy
	go mod vendor
build:
	cp -f $(SSH_KEY_PATH) id_rsa
	docker build -t $(IMAGE):latest .
	rm id_rsa
up:
	docker-compose up -d
down:
	docker-compose down
clean: down
	rm -rf $(BINARY)
	docker rmi -f $(shell docker images -f "dangling=true" -q) 2> /dev/null; true
	docker rmi -f $(IMAGE):latest 2> /dev/null; true
