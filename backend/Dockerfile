# Backend Dockerfile for Match-Me Go server
FROM golang:1.25-alpine AS builder

# Set working directory
WORKDIR /app

# Install git (needed for go modules)
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS calls
RUN apk --no-cache add ca-certificates

# Create app directory
WORKDIR /app/

# Copy the binary from builder stage
COPY --from=builder /app/main .

# Create uploads directory structure and copy placeholder avatar
RUN mkdir -p uploads/avatars
COPY avatar-placeholder.png ./uploads/avatars/avatar_placeholder.png

# Expose port
EXPOSE 8080

# Run the application
CMD ["./main"]
