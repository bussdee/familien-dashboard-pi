# syntax=docker/dockerfile:1

# ---- build ----------------------------------------------------------------
# modernc.org/sqlite is pure Go, so CGO stays off and the same source
# cross-compiles for the Pi (arm64) without a toolchain.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/server .

# ---- runtime --------------------------------------------------------------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget \
    && adduser -D -u 1000 app

WORKDIR /app
COPY --from=builder /out/server /app/server
COPY config.yaml /app/config.yaml

# /app/data is a mounted volume; create it so a first run on an empty host works.
RUN mkdir -p /app/data && chown -R app:app /app

USER app
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/health >/dev/null || exit 1

ENTRYPOINT ["/app/server", "-config", "/app/config.yaml"]
