# --Build stage 
FROM golang:1.25-alpine AS builder

#intall git (needed for go modules sometimes)\
RUN apk add --no-cache git

#set working dir inside container
WORKDIR /app

# copy go.mod and go.sum first (layer caching)
COPY go.mod go.sum ./
RUN go mod download

# copy the rest of the source code
COPY . .

#build the admin-service binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
go build -o admin-service ./services/admin-service/cmd

# --- Runtime stage 
FROM alpine:3.20
#create a non-root user (basic security measure)

RUN apk add --no-cache ca-certificates
RUN adduser -D appuser

WORKDIR /app

#copy binary from builder stage
COPY --from=builder /app/admin-service .

# user non root user
USER appuser

#expose port for documentation and not for networking 
EXPOSE 8080

#run binary
ENTRYPOINT [ "./admin-service" ]
