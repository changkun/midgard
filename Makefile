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

all:
	go build $(TARGET) $(BUILD_FLAGS)
dep:
	go mod tidy
build:
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):latest .
up:
	docker-compose up -d
down:
	docker-compose down
mac: # the Mac app, into apple/build: Midgard.app, and Midgard.dmg
	./apple/build.sh
dmg: # the same, for Apple silicon and Intel both, as the download is
	./apple/build.sh universal
clean: down
	rm -rf $(BINARY)
	docker rmi -f $(shell docker images -f "dangling=true" -q) 2> /dev/null; true
	docker rmi -f $(IMAGE):latest 2> /dev/null; true
