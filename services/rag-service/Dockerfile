# ==============================================================================
# Stage 1: build
# ==============================================================================
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETARCH

RUN apk add --no-cache protobuf protobuf-dev git

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
    go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY proto/ proto/

RUN protoc \
    --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    --grpc-gateway_out=. --grpc-gateway_opt=paths=source_relative \
    proto/llm/v1/llm.proto \
    proto/rag/v1/rag.proto \
    proto/embedder/v1/embedder.proto

COPY services/rag-service/ services/rag-service/

RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build \
    -ldflags="-w -s" \
    -o /bin/rag-service \
    ./services/rag-service/cmd/...

# ==============================================================================
# Stage 2: final — distroless, no shell, non-root
# To debug: kubectl debug -it deployment/rag-service --image=alpine --target=rag-service -n jobradar
# ==============================================================================
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /bin/rag-service /rag-service

EXPOSE 50052

ENTRYPOINT ["/rag-service"]
