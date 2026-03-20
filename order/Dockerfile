# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/order ./cmd/order

FROM alpine:3.20
WORKDIR /app
RUN adduser -D appuser
COPY --from=builder /bin/order /app/order
COPY --from=builder /src/internal/platform/config/config.yaml /app/config.yaml
USER appuser
EXPOSE 8080
ENTRYPOINT ["/app/order"]