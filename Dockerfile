# Copyright 2020-2021 Changkun Ou. All rights reserved.
# Use of this source code is governed by a GPL-3.0
# license that can be found in the LICENSE file.

FROM chromedp/headless-shell:latest AS builder-env
WORKDIR /app
COPY . .
RUN apt update && apt install -y wget gcc xclip libx11-dev
RUN mkdir -p /root/goes
RUN cd /root/goes && wget https://dl.google.com/go/go1.16.linux-amd64.tar.gz 
RUN cd /root/goes && tar xvf go1.16.linux-amd64.tar.gz && rm go1.16.linux-amd64.tar.gz 
RUN cd /root/goes && mv /root/goes/go /root/goes/go1.16 
RUN cd /root/goes && ln -s /root/goes/go1.16 /root/goes/go 
RUN cd /root/goes && export GOROOT=~/goes/go
RUN /root/goes/go/bin/go build -mod=vendor

FROM chromedp/headless-shell:latest
RUN apt update && apt install -y dumb-init git xclip libx11-dev
ENTRYPOINT ["dumb-init", "--"]

WORKDIR /app
COPY . .
COPY --from=builder-env /app/midgard /app/mg
RUN mkdir -p /root/.ssh && \
  mv id_rsa /root/.ssh/id_rsa && \
  chmod 400 /root/.ssh/id_rsa && \
  echo "StrictHostKeyChecking no" > /root/.ssh/config && \
  git config --global url."git@github.com:".insteadOf "https://github.com/" && \
  git config --global user.name "Changkun Ou" && \
  git config --global user.email "hi@changkun.de"
EXPOSE 80
CMD ["/app/mg", "server"]
