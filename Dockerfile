ARG DOCKER_HUB="docker.io"

# The wasm module and the generated command spec are inputs to the frontend build.
FROM ${DOCKER_HUB}/golang:1.26-alpine AS wasm

RUN apk add --no-cache git
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN cd web && ./export-go-data.sh && ./build-wasm.sh

FROM ${DOCKER_HUB}/node:24-alpine AS frontend

WORKDIR /build
COPY web/package.json web/package-lock.json ./web/
RUN cd web && npm ci --include=dev
COPY . .
COPY --from=wasm /build/web/src/api/wasm/_gomuks.wasm ./web/src/api/wasm/
COPY --from=wasm /build/web/src/api/types/stdcommands.json /build/web/src/api/types/stdcommands.d.ts ./web/src/api/types/
RUN cd web && npm run build

FROM ${DOCKER_HUB}/golang:1.26-alpine AS builder

# build-base is needed for the cgo sqlite driver.
RUN apk add --no-cache git build-base
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /build/web/dist ./web/dist

# goolm replaces libolm with a pure Go implementation, so no C crypto library is needed.
# build.sh isn't used because it ignores frontend build failures.
ENV GO_BUILD_TAGS=goolm
RUN ./build-noweb.sh

FROM ${DOCKER_HUB}/alpine:3.23

RUN apk add --no-cache ca-certificates jq curl ffmpeg

COPY --from=builder /build/gomuks /usr/bin/gomuks
VOLUME /data
WORKDIR /data
ENV GOMUKS_ROOT=/data

CMD ["/usr/bin/gomuks"]
