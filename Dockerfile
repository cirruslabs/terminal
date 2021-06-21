FROM golang:latest AS builder

WORKDIR /build
COPY . .

RUN CGO_ENABLED=0 go build cmd/terminal/terminal.go

FROM alpine:latest

COPY --from=builder /build/terminal /usr/local/bin/terminal

ENTRYPOINT ["/usr/local/bin/terminal"]
