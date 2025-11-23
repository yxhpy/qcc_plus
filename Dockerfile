FROM golang:1.21 AS build
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/ccproxy ./cmd/cccli

# 下载 cloudflared
FROM debian:bookworm-slim AS cloudflared
RUN apt-get update && apt-get install -y curl && \
    curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 -o /cloudflared && \
    chmod +x /cloudflared

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /app/ccproxy /usr/local/bin/ccproxy
COPY --from=cloudflared /cloudflared /usr/local/bin/cloudflared
EXPOSE 8000
ENTRYPOINT ["/usr/local/bin/ccproxy", "proxy"]
