# ==============================================================================
# Stage 1: proto generation
# ==============================================================================
FROM golang:1.26-alpine AS proto-builder

RUN apk add --no-cache protobuf protobuf-dev

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY proto/ proto/

RUN protoc \
    --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    --grpc-gateway_out=. --grpc-gateway_opt=paths=source_relative \
    proto/llm/v1/llm.proto \
    proto/rag/v1/rag.proto \
    proto/embedder/v1/embedder.proto

# ==============================================================================
# Stage 2: build
# ==============================================================================
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY --from=proto-builder /app/go.mod /app/go.sum ./
COPY --from=proto-builder /app/proto/ proto/

RUN go mod download

COPY services/embedder/ services/embedder/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s" \
    -o /bin/embedder \
    ./services/embedder/cmd/...

# ==============================================================================
# Stage 3: final image
# ==============================================================================
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/embedder /embedder

EXPOSE 50051

ENTRYPOINT ["/embedder"]