ARG GO_VERSION=1.24.5
FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download && go mod verify
COPY . .
RUN go build -v -o /app/smoke_test ./cmd/smoke_test

FROM alpine:3

COPY --from=builder /app/smoke_test /usr/local/bin/
CMD ["smoke_test"]
