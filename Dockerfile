# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build API binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api ./cmd/api

# Build CLI binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o chat-cli ./cmd/chat-cli

# Build seeder binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o seed-vectors ./cmd/seed-vectors

# Runtime stage for API
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binaries from builder
COPY --from=builder /app/api .
COPY --from=builder /app/chat-cli .
COPY --from=builder /app/seed-vectors .
COPY config.yaml .

# Default to API server
ENV CONFIG_PATH=config.yaml

EXPOSE 8080

CMD ["./api"]
