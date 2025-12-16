FROM golang:1.21-alpine AS builder

ENV GO111MODULE=on
ENV CGO_ENABLED=0

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -v -o marketflow ./cmd/marketflow

# --------------------

FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /root

COPY --from=builder /app/marketflow .
COPY --from=builder /app/configs ./configs

EXPOSE 8080

CMD ["./marketflow"]
