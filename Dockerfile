# Copyright 2020-2021 Changkun Ou. All rights reserved.
# Use of this source code is governed by a GPL-3.0
# license that can be found in the LICENSE file.

# The server needs no Cgo, so it is built static. Nothing secret goes into
# the image: the configuration is mounted when the container runs (see
# docker-compose.yml).
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=devel
RUN CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X changkun.de/x/midgard/internal/version.GitVersion=${VERSION}" \
  -o /out/mg .

# A static binary needs no distribution: only CA certificates, for the
# HTTPS the server makes, which distroless/static carries. There is no git
# any more (backups are the data volume's) and no browser (code2img is gone).
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/mg /app/mg
ENV MIDGARD_CONF=/app/config.yml
EXPOSE 80
ENTRYPOINT ["/app/mg", "server"]
