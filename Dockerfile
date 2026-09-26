# Copyright 2020-2021 Changkun Ou. All rights reserved.
# Use of this source code is governed by a GPL-3.0
# license that can be found in the LICENSE file.

# The server needs no Cgo, so it is built static. Nothing secret goes into
# the image: the configuration and, for backups over ssh, a key are mounted
# when the container runs (see docker-compose.yml).
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=devel
RUN CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X changkun.de/x/midgard/internal/version.GitVersion=${VERSION}" \
  -o /out/mg .

# headless-shell is the Chrome that code2img renders with; git is for backups.
FROM chromedp/headless-shell:latest
RUN apt-get update && \
  apt-get install -y --no-install-recommends dumb-init git openssh-client ca-certificates && \
  rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /out/mg /app/mg
COPY data/template /app/data/template
# Backup commits need an author; set your own in docker-compose.yml.
ENV MIDGARD_CONF=/app/config.yml \
  GIT_AUTHOR_NAME=midgard GIT_AUTHOR_EMAIL=midgard@localhost \
  GIT_COMMITTER_NAME=midgard GIT_COMMITTER_EMAIL=midgard@localhost
EXPOSE 80
ENTRYPOINT ["dumb-init", "--"]
CMD ["/app/mg", "server"]
