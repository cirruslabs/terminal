FROM golang:latest AS builder

WORKDIR /build
COPY . .

RUN CGO_ENABLED=0 go build cmd/terminal/terminal.go

FROM alpine:latest

LABEL org.opencontainers.image.source=https://github.com/cirruslabs/terminal

COPY --from=builder /build/terminal /usr/local/bin/terminal

ENTRYPOINT ["/usr/local/bin/terminal"]
