FROM golang:1.21 AS build
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/ccproxy ./cmd/cccli

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /app/ccproxy /usr/local/bin/ccproxy
EXPOSE 8000
ENTRYPOINT ["/usr/local/bin/ccproxy", "proxy"]
