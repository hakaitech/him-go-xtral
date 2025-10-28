FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy everything including vendored dependencies
COPY . .

# Build the application using vendor directory
RUN go build -mod=vendor -o main .

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/main .

# Expose port
EXPOSE 8080

# Run the application
CMD ["./main"]
