# Build stage
FROM golang:1.24.3-alpine AS builder

# Force rebuild
RUN echo "Build time: $(date)"

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/api-server/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests (if needed)
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Expose port
EXPOSE 3000

# Command to run
CMD ["./main"]