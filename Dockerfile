ARG GO_VERSION=1.24.5

FROM golang:${GO_VERSION}-alpine AS builder
ARG TARGET_BINARY=smoke_test

WORKDIR /app
COPY go.mod ./
RUN go mod download && go mod verify
COPY . .
RUN go build -v -o /app/out ./cmd/$TARGET_BINARY

FROM alpine:3

COPY --from=builder /app/out /usr/local/bin/app
CMD ["/usr/local/bin/app"]
