# ==============================================================================
# Stage 1: build
# ==============================================================================
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY db/migrate/ db/migrate/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build \
    -ldflags="-w -s" \
    -o /bin/migrate \
    ./db/migrate/cmd/...

# ==============================================================================
# Stage 2: final
# Migrations are embedded in the binary via go:embed
# ==============================================================================
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/migrate /migrate
COPY db/migrations /migrations

ENTRYPOINT ["/migrate"]
CMD ["up"]