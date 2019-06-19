# Copyright (c) JSC iCore.

# This source code is licensed under the MIT license found in the
# LICENSE file in the root directory of this source tree.

FROM golang:1.12-alpine AS build

ARG VERSION
ARG GOPROXY

WORKDIR /opt/build

RUN adduser -D -g '' appuser
COPY go.mod .
COPY go.sum .
COPY cmd cmd
COPY internal internal
RUN env CGO_ENABLED=0 go install -ldflags="-w -s -X main.version=${VERSION}" ./...

FROM alpine:edge AS final

RUN apk add --no-cache chromium
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /go/bin/tokget /usr/local/bin/
COPY run.sh /usr/local/bin/

USER appuser

ENTRYPOINT ["/usr/local/bin/run.sh"]
