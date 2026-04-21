FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY ./cmd ./cmd
COPY ./web ./web
COPY ./tasks ./tasks

RUN go build -o server ./cmd/server/main.go

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/server .
COPY --from=builder /app/web ./web

CMD ["./server"]
