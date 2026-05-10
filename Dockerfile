# syntax=docker/dockerfile:1

# ── build stage ──────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

WORKDIR /src

# Cache module downloads separately from source changes
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build both binaries; CGO disabled for a fully static binary
RUN CGO_ENABLED=0 go build -trimpath -o /out/swiggy-ssh      ./cmd/swiggy-ssh
RUN CGO_ENABLED=0 go build -trimpath -o /out/swiggy-migrate  ./cmd/swiggy-ssh-migrate

# ── runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.20

# ca-certs for any future TLS calls; tzdata for log timestamps
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /out/swiggy-ssh     ./swiggy-ssh
COPY --from=builder /out/swiggy-migrate ./swiggy-migrate

# .local/ is where the SSH host key is stored; mount a volume here in production
RUN mkdir -p .local && chmod 700 .local

EXPOSE 2222 8080

CMD ["./swiggy-ssh"]
