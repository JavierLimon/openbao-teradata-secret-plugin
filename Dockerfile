FROM docker.io/library/golang:1.24-alpine3.21 AS builder

RUN apk add --no-cache git make

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -o teradata-plugin ./cmd/teradata/

FROM docker.io/library/alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /plugin

COPY --from=builder /build/teradata-plugin .

RUN chmod +x teradata-plugin

ENV TERADATA_PLUGIN_VERSION=0.1.0

ENTRYPOINT ["/plugin/teradata-plugin"]
