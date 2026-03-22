# ==============================================================================
# JobRadar Makefile
# ==============================================================================

.DEFAULT_GOAL := help
.PHONY: help cluster-up cluster-down cluster-status proto proto-clean build \
        build-service deploy-infra deploy-observability dev dev-run \
        dev-service dev-delete test test-unit test-integration \
        test-coverage test-service lint fmt vet tidy clean

# ------------------------------------------------------------------------------
# Configuration
# ------------------------------------------------------------------------------
include .env
export

ENV       ?= local
NAMESPACE := jobradar
CLUSTER   := jobradar

SERVICES := auth fetcher scorer llm-service rag-service embedder api-gateway mcp-server a2a-agent

# ------------------------------------------------------------------------------
# Help
# ------------------------------------------------------------------------------
help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# ------------------------------------------------------------------------------
# Cluster (kind)
# ------------------------------------------------------------------------------
cluster-up: ## Create local kind cluster
	@echo "→ Creating kind cluster '$(CLUSTER)'..."
	kind create cluster --name $(CLUSTER) --config infra/local/kind-cluster.yaml
	kubectl create namespace $(NAMESPACE) || true
	@echo "✓ Cluster ready"

cluster-down: ## Delete local kind cluster
	@echo "→ Deleting kind cluster '$(CLUSTER)'..."
	kind delete cluster --name $(CLUSTER)
	@echo "✓ Cluster deleted"

cluster-status: ## Show cluster and pod status
	kubectl get nodes
	kubectl get pods -n $(NAMESPACE)

# ------------------------------------------------------------------------------
# Protobuf
# ------------------------------------------------------------------------------
proto: ## Generate Go code from .proto files
	@echo "→ Generating protobuf stubs..."
	@which protoc > /dev/null || (echo "✗ protoc not found. Install: https://grpc.io/docs/protoc-installation/" && exit 1)
	@which protoc-gen-go > /dev/null || go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@which protoc-gen-go-grpc > /dev/null || go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@which protoc-gen-grpc-gateway > /dev/null || go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	protoc \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=. --grpc-gateway_opt=paths=source_relative \
		proto/llm/v1/llm.proto \
		proto/rag/v1/rag.proto \
		proto/embedder/v1/embedder.proto
	@echo "✓ Protobuf stubs generated"

proto-clean: ## Remove generated protobuf files
	find proto -name "*.pb.go" -delete
	find proto -name "*_grpc.pb.go" -delete
	@echo "✓ Generated proto files removed"

# ------------------------------------------------------------------------------
# Build (local, outside K8s — optional, requires Go installed)
# ------------------------------------------------------------------------------
build: ## Build all Go services locally
	@echo "→ Building Go services..."
	@for svc in $(SERVICES); do \
		echo "  building $$svc..."; \
		go build -o bin/$$svc ./services/$$svc/cmd/...; \
	done
	@echo "✓ All services built"

build-service: ## Build a single service locally (make build-service SVC=embedder)
	@[ -n "$(SVC)" ] || (echo "✗ SVC is required. Usage: make build-service SVC=embedder" && exit 1)
	go build -o bin/$(SVC) ./services/$(SVC)/cmd/...
	@echo "✓ $(SVC) built"

# ------------------------------------------------------------------------------
# Infrastructure
# ------------------------------------------------------------------------------
deploy-infra: ## Deploy infrastructure (PostgreSQL, Kafka, Valkey, MinIO, LiteLLM, Langfuse, LGTM)
	@echo "→ Deploying infrastructure (ENV=$(ENV))..."
	helm repo add bitnami https://charts.bitnami.com/bitnami || true
	helm repo add grafana https://grafana.github.io/helm-charts || true
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts || true
	helm repo add langfuse https://langfuse.github.io/langfuse-k8s || true
	helm repo update
	helm upgrade --install postgresql bitnami/postgresql \
		-n $(NAMESPACE) \
		-f k8s/helm/postgresql/values.yaml \
		--set auth.password=$(POSTGRES_PASSWORD) \
		--set global.security.allowInsecureImages=true
	kubectl apply -f k8s/manifests/kafka/ -n $(NAMESPACE)
	helm upgrade --install valkey bitnami/valkey \
		-n $(NAMESPACE) \
		-f k8s/helm/valkey/values.yaml
	kubectl apply -f k8s/manifests/minio/ -n $(NAMESPACE)
	helm upgrade --install litellm oci://ghcr.io/berriai/litellm-helm \
		-n $(NAMESPACE) \
		-f k8s/helm/litellm/values.yaml
	helm upgrade --install langfuse langfuse/langfuse \
		-n $(NAMESPACE) \
		-f k8s/helm/langfuse/values.yaml
	$(MAKE) deploy-observability
	@echo "✓ Infrastructure deployed"

deploy-observability: ## Deploy LGTM observability stack
	@echo "→ Deploying observability stack..."
	helm upgrade --install alloy grafana/alloy \
		-n $(NAMESPACE) \
		-f k8s/helm/observability/alloy/values.yaml
	helm upgrade --install tempo grafana/tempo \
		-n $(NAMESPACE) \
		-f k8s/helm/observability/tempo/values.yaml
	helm upgrade --install loki grafana/loki \
		-n $(NAMESPACE) \
		-f k8s/helm/observability/loki/values.yaml
	helm upgrade --install prometheus prometheus-community/prometheus \
		-n $(NAMESPACE) \
		-f k8s/helm/observability/prometheus/values.yaml
	helm upgrade --install grafana grafana/grafana \
		-n $(NAMESPACE) \
		-f k8s/helm/observability/grafana/values.yaml
	@echo "✓ Observability stack deployed"

# ------------------------------------------------------------------------------
# Development — Skaffold
# ------------------------------------------------------------------------------
dev: cluster-up deploy-infra ## Full local setup + deploy all services (watch mode)
	@echo "→ Starting Skaffold dev..."
	@echo ""
	@echo "  Frontend:  http://localhost:5173"
	@echo "  API:       http://localhost:8080"
	@echo "  MCP:       http://localhost:8090/mcp"
	@echo "  Grafana:   http://localhost:3100"
	@echo "  Langfuse:  http://localhost:3101"
	@echo ""
	skaffold dev --profile local

dev-run: ## Deploy all services once without watch mode
	skaffold run --profile local

dev-service: ## Watch and redeploy a single service (make dev-service SVC=embedder)
	@[ -n "$(SVC)" ] || (echo "✗ SVC is required. Usage: make dev-service SVC=embedder" && exit 1)
	skaffold dev --profile local --build-artifacts $(SVC)

port-forward-infra: ## Forward all infrastructure ports to localhost
	@echo "→ Forwarding infrastructure ports..."
	kubectl port-forward svc/postgresql        5432:5432  -n $(NAMESPACE) &
	kubectl port-forward svc/kafka             9092:9092  -n $(NAMESPACE) &
	kubectl port-forward svc/valkey            6379:6379  -n $(NAMESPACE) &
	kubectl port-forward svc/minio             9000:9000  -n $(NAMESPACE) &
	kubectl port-forward svc/minio             9001:9001  -n $(NAMESPACE) &
	kubectl port-forward svc/litellm           4000:4000  -n $(NAMESPACE) &
	kubectl port-forward svc/langfuse-web      3101:3000  -n $(NAMESPACE) &
	kubectl port-forward svc/grafana           3100:3000  -n $(NAMESPACE) &
	kubectl port-forward svc/tempo             3200:3200  -n $(NAMESPACE) &
	kubectl port-forward svc/loki              3102:3100  -n $(NAMESPACE) &
	kubectl port-forward svc/prometheus-server 9090:9090  -n $(NAMESPACE) &
	kubectl port-forward svc/alloy             4317:4317  -n $(NAMESPACE) &
	@echo ""
	@echo "  PostgreSQL:  localhost:5432"
	@echo "  Kafka:       localhost:9092"
	@echo "  Valkey:      localhost:6379"
	@echo "  MinIO API:   http://localhost:9000"
	@echo "  MinIO UI:    http://localhost:9001  (minioadmin / jobradar_local)"
	@echo "  LiteLLM:     http://localhost:4000  (Bearer sk-jobradar)"
	@echo "  Langfuse:    http://localhost:3101  (create on first access)"
	@echo "  Grafana:     http://localhost:3100  (admin / admin)"
	@echo "  Tempo:       http://localhost:3200"
	@echo "  Loki:        http://localhost:3102"
	@echo "  Prometheus:  http://localhost:9090"
	@echo "  OTel/Alloy:  localhost:4317"
	@echo ""
	@echo "✓ Ports forwarded — use 'make port-forward-stop' to stop"

port-forward-stop: ## Stop all port-forwards
	pkill -f "kubectl port-forward" || true
	@echo "✓ Port forwards stopped"

dev-delete: ## Remove all Skaffold-managed resources
	skaffold delete --profile local

# ------------------------------------------------------------------------------
# Testing
# ------------------------------------------------------------------------------
test: test-unit test-integration ## Run all tests

test-unit: ## Run unit tests
	@echo "→ Running unit tests..."
	go test -v -race -count=1 ./services/.../internal/...
	@echo "✓ Unit tests passed"

test-integration: ## Run integration tests (requires running cluster)
	@echo "→ Running integration tests..."
	go test -v -race -count=1 -tags=integration ./services/.../...
	@echo "✓ Integration tests passed"

test-coverage: ## Run tests with coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

test-service: ## Run tests for a single service (make test-service SVC=embedder)
	@[ -n "$(SVC)" ] || (echo "✗ SVC is required. Usage: make test-service SVC=embedder" && exit 1)
	go test -v -race -count=1 ./services/$(SVC)/...

# ------------------------------------------------------------------------------
# Code quality
# ------------------------------------------------------------------------------
lint: ## Run golangci-lint
	@which golangci-lint > /dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run ./...

fmt: ## Format Go code
	gofmt -w ./services/...
	goimports -w ./services/...

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy Go modules
	go mod tidy

# ------------------------------------------------------------------------------
# Clean
# ------------------------------------------------------------------------------
clean: ## Remove build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html
	$(MAKE) proto-clean
	@echo "✓ Clean done"