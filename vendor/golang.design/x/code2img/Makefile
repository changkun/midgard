# Copyright 2020 The golang.design Initiative authors.
# All rights reserved. Use of this source code is governed
# by a GNU GPL-3.0 license that can be found in the LICENSE file.

all:
	GOOS=linux go build -mod=vendor ./cmd/code2img
	docker build -t code2img -f docker/Dockerfile .
up:
	docker-compose -f docker/docker-compose.yml up -d
down:
	docker-compose -f docker/docker-compose.yml down
clean:
	rm code2img
	docker rmi -f $(shell docker images -f "dangling=true" -q) 2> /dev/null; true
.PHONY: up down clean