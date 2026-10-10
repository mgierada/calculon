ARG GO_VERSION=1
FROM golang:${GO_VERSION}-bookworm AS builder

WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN CGO_ENABLED=0 go build -v -o /calculon ./cmd/calculon

FROM alpine:3.24

COPY --from=builder /calculon /usr/local/bin/

# Mount the directory holding calculon.db and .ssh/ here to keep them across runs.
WORKDIR /data
ENV DB_PATH=/data/calculon.db \
    SSH_HOST_KEY=/data/.ssh/calculon_ed25519 \
    SSH_ADDR=:23234
EXPOSE 23234
CMD ["calculon", "serve"]
