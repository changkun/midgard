# Copyright 2020 Changkun Ou. All rights reserved.
# Use of this source code is governed by a GPL-3.0
# license that can be found in the LICENSE file.

FROM chromedp/headless-shell:latest
RUN apt update && apt install -y dumb-init git xclip libx11-dev
ENTRYPOINT ["dumb-init", "--"]
WORKDIR /app
COPY . .
CMD ["/app/mg", "server", "run"]
