# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY backend/go.mod backend/go.sum* ./

# Download dependencies
RUN go mod download

# Copy backend source
COPY backend/ .

# Build minimal binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o network-view-osx .

# Runtime stage
FROM scratch

COPY --from=builder /build/network-view-osx /app/network-view-osx

# Create /etc/passwd for nobody user
COPY --from=builder /etc/passwd /etc/passwd

USER nobody

EXPOSE 8080

ENTRYPOINT ["/app/network-view-osx"]
