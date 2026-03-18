# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
go build -o admin-service ./services/admin-service/cmd

# Final stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates
RUN adduser -D appuser

WORKDIR /app

COPY --from=builder /app/admin-service .

USER appuser

EXPOSE 8080

CMD ["./admin-service"]
