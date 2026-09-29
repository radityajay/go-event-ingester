# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /ingester ./cmd/ingester

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /ingester /ingester

EXPOSE 8080

ENTRYPOINT ["/ingester"]
