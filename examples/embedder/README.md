# Examples — Embedder Service

Demonstrates how to interact with the `embedder` gRPC service.

## Prerequisites

```bash
# Install grpcurl
brew install grpcurl

# Forward embedder port
kubectl port-forward svc/embedder 50051:50051 -n jobradar
```

## Examples

### Embed a single text

```bash
./embed_text.sh "Staff Backend Engineer Go Kubernetes"
```

Use `query` purpose when searching against stored documents:

```bash
./embed_text.sh "Go jobs in Barcelona" query
```

### Embed a batch of texts

```bash
./embed_batch.sh
```

### Embed a CV

```bash
./embed_cv.sh "my-profile-id" "10 years experience in Go, Kubernetes..."
```

## Expected output

```
✓ Embedding generated
  Model:      embeddings
  Dimensions: 1024
  Tokens:     8
  Latency:    45ms
  Preview:    [-0.0197, -0.0055, -0.0837, -0.0324, -0.0028] ...
```

## Connect to a different host

By default the scripts connect to `localhost:50051`. To connect to a remote host:

```bash
EMBEDDER_HOST=embedder.yourdomain.com:50051 ./embed_text.sh "Go Kubernetes"
```