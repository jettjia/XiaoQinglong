# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o runner .

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy binary
COPY --from=builder /workspace/runner /app/runner

# Copy skills templates (will be used if XQL_SOURCE_SKILLS_DIR=/app/skills)
COPY --from=builder /workspace/skills /app/skills

EXPOSE 18080

ENTRYPOINT ["/app/runner"]
