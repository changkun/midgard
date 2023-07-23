# Copyright 2020-2021 Changkun Ou. All rights reserved.
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
BUILD_FLAGS = $(BUILD_SETTINGS) -x -work
GOVERSION = $(shell curl -s 'https://go.dev/dl/?mode=json' | grep '"version"' | sed 1q | awk '{print $$2}' | tr -d ',"') # get latest go version

all:
	go build $(TARGET) $(BUILD_FLAGS)
install:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.27.1
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
gen:
	go generate ./...
dep:
	go mod tidy
	go mod vendor
build:
	cp -f $(SSH_KEY_PATH) id_rsa
	docker build --build-arg GOVERSION=$(GOVERSION) -t $(IMAGE):latest .
	rm id_rsa
up:
	docker-compose up -d
down:
	docker-compose down
clean: down
	rm -rf $(BINARY)
	docker rmi -f $(shell docker images -f "dangling=true" -q) 2> /dev/null; true
	docker rmi -f $(IMAGE):latest 2> /dev/null; true
