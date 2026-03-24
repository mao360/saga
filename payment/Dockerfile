FROM golang:1.25-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/payment ./cmd/payment

FROM alpine:3.20
WORKDIR /app
RUN adduser -D appuser
COPY --from=builder /bin/payment /app/payment
COPY --from=builder /src/internal/platform/config/config.yaml /app/config.yaml
COPY --from=builder /src/migrations /app/migrations
USER appuser
EXPOSE 8083
ENTRYPOINT ["/app/payment"]
