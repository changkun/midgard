# Copyright 2020 Changkun Ou. All rights reserved.
# Use of this source code is governed by a GPL-3.0
# license that can be found in the LICENSE file.

FROM chromedp/headless-shell:latest
RUN apt update && apt install -y dumb-init git xclip libx11-dev
ENTRYPOINT ["dumb-init", "--"]

WORKDIR /app
COPY . .
RUN mkdir -p /root/.ssh && \
  mv id_rsa /root/.ssh/id_rsa && \
  chmod 400 /root/.ssh/id_rsa && \
  echo "StrictHostKeyChecking no" > /root/.ssh/config && \
  git config --global url."git@github.com:".insteadOf "https://github.com/" && \
  git config --global user.name "Changkun Ou" && \
  git config --global user.email "hi@changkun.de"
CMD ["/app/mg", "server"]
