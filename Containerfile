# Stage 1: Build static Go binary with embedded Web UI and LICENSE
FROM docker.io/library/golang:1.23-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY web/ ./web/
COPY LICENSE ./LICENSE

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/lan-video-player ./cmd/lan-video-player

# Stage 2: Minimal non-root runtime compatible with Podman rootless and Docker
FROM docker.io/library/alpine:3.20

RUN apk add --no-cache ffmpeg ca-certificates tzdata && \
    addgroup -g 10001 -S lvp && \
    adduser -u 10001 -S -G lvp -h /config lvp && \
    mkdir -p /media/home /media/mnt /config /tmp/lan-video-player-thumbs && \
    chown -R 10001:10001 /media /config /tmp/lan-video-player-thumbs

WORKDIR /app
COPY --from=builder /out/lan-video-player /usr/local/bin/lan-video-player
COPY --from=builder /src/LICENSE /app/LICENSE

USER 10001:10001

ENV LVP_ADDR="0.0.0.0:8080" \
    LVP_BROWSE_BASE="/media" \
    LVP_CONFIG_FILE="/config/library.json" \
    LVP_CACHE_DIR="/tmp/lan-video-player-thumbs"

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/lan-video-player"]
