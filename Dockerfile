# Build stage with garble for obfuscation
FROM golang:1.23-alpine AS builder

# Install garble for binary obfuscation and ca-certificates for HTTPS
RUN apk add --no-cache git ca-certificates tzdata && \
    go install mvdan.cc/garble@latest

WORKDIR /src

# Copy dependency files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build obfuscated static binary
# -s: strip symbol table
# -w: strip DWARF debugging info
# garble flags:
#   -literals: obfuscate string literals
#   -tiny: optimize for size over debuggability
#   -seed=random: use random seed for each build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    garble -literals -tiny -seed=random build \
    -ldflags="-s -w" \
    -trimpath \
    -o /regrada .

# Final stage - scratch image (smallest possible, ~0 bytes base)
FROM scratch

# Copy CA certificates for HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data for time operations
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the obfuscated binary
COPY --from=builder /regrada /regrada

# Run as non-root (UID 65534 is 'nobody')
USER 65534:65534

ENTRYPOINT ["/regrada"]
