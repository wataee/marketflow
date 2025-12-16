FROM golang:1.21-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o marketflow ./cmd/marketflow

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/marketflow .
COPY --from=builder /app/configs ./configs

EXPOSE 8080

CMD ["./marketflow"]