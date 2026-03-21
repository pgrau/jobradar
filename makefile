# ==============================================================================
# JobRadar Makefile
# ==============================================================================

.DEFAULT_GOAL := help
.PHONY: help cluster-up cluster-down proto build images-build images-load-kind \
        deploy-infra deploy deploy-service undeploy logs port-forward \
        test test-unit test-integration lint fmt vet tidy clean

# ------------------------------------------------------------------------------
# Configuration
# ------------------------------------------------------------------------------
include .env
-include .env.$(ENV)
export

REGISTRY   := ghcr.io/pgrau/jobradar
TAG        ?= latest
ENV        ?= local
NAMESPACE  := jobradar
CLUSTER    := jobradar

SERVICES := auth fetcher scorer llm-service rag-service embedder api-gateway mcp-server a2a-agent
FRONTEND := apps/frontend

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
# Build
# ------------------------------------------------------------------------------
build: proto ## Build all Go services
	@echo "→ Building Go services..."
	@for svc in $(SERVICES); do \
		echo "  building $$svc..."; \
		go build -o bin/$$svc ./services/$$svc/cmd/...; \
	done
	@echo "✓ All services built"

build-service: proto ## Build a single service (make build-service SVC=fetcher)
	@[ -n "$(SVC)" ] || (echo "✗ SVC is required. Usage: make build-service SVC=fetcher" && exit 1)
	@echo "→ Building $(SVC)..."
	go build -o bin/$(SVC) ./services/$(SVC)/cmd/...
	@echo "✓ $(SVC) built"

# ------------------------------------------------------------------------------
# Docker images
# ------------------------------------------------------------------------------
images-build: ## Build Docker images for all services
	@echo "→ Building Docker images (ENV=$(ENV))..."
	@for svc in $(SERVICES); do \
		echo "  building $(REGISTRY)/$$svc:$(TAG)..."; \
		docker build \
			--build-arg SERVICE=$$svc \
			--build-arg ENV=$(ENV) \
			-t $(REGISTRY)/$$svc:$(TAG) \
			-f services/$$svc/Dockerfile .; \
	done
	docker build \
		-t $(REGISTRY)/frontend:$(TAG) \
		-f $(FRONTEND)/Dockerfile $(FRONTEND)/
	@echo "✓ All images built"

images-build-service: ## Build a single service image (make images-build-service SVC=fetcher)
	@[ -n "$(SVC)" ] || (echo "✗ SVC is required. Usage: make images-build-service SVC=fetcher" && exit 1)
	docker build \
		--build-arg SERVICE=$(SVC) \
		--build-arg ENV=$(ENV) \
		-t $(REGISTRY)/$(SVC):$(TAG) \
		-f services/$(SVC)/Dockerfile .
	@echo "✓ $(SVC) image built"

images-load-kind: ## Load all images into kind cluster
	@echo "→ Loading images into kind cluster '$(CLUSTER)'..."
	@for svc in $(SERVICES); do \
		echo "  loading $(REGISTRY)/$$svc:$(TAG)..."; \
		kind load docker-image $(REGISTRY)/$$svc:$(TAG) --name $(CLUSTER); \
	done
	kind load docker-image $(REGISTRY)/frontend:$(TAG) --name $(CLUSTER)
	@echo "✓ All images loaded"

images-push: ## Push all images to registry (ghcr.io)
	@echo "→ Pushing images to $(REGISTRY)..."
	@for svc in $(SERVICES); do \
		docker push $(REGISTRY)/$$svc:$(TAG); \
	done
	docker push $(REGISTRY)/frontend:$(TAG)
	@echo "✓ All images pushed"

# ------------------------------------------------------------------------------
# Deploy
# ------------------------------------------------------------------------------
deploy-infra: ## Deploy infrastructure (Kafka, PostgreSQL, Valkey, MinIO, LiteLLM, Langfuse, LGTM)
	@echo "→ Deploying infrastructure (ENV=$(ENV))..."
	helm repo add bitnami https://charts.bitnami.com/bitnami || true
	helm repo add grafana https://grafana.github.io/helm-charts || true
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts || true
	helm repo update
	helm upgrade --install postgresql bitnami/postgresql -n $(NAMESPACE) \
		-f k8s/helm/postgresql/values.yaml \
		-f k8s/helm/postgresql/values.$(ENV).yaml \
		--set auth.password=$(POSTGRES_PASSWORD)
	helm upgrade --install kafka bitnami/kafka -n $(NAMESPACE) \
		-f k8s/helm/kafka/values.yaml \
		-f k8s/helm/kafka/values.$(ENV).yaml
	helm upgrade --install valkey bitnami/valkey -n $(NAMESPACE) \
		-f k8s/helm/valkey/values.yaml \
		-f k8s/helm/valkey/values.$(ENV).yaml
	helm upgrade --install minio bitnami/minio -n $(NAMESPACE) \
		-f k8s/helm/minio/values.yaml \
		-f k8s/helm/minio/values.$(ENV).yaml \
		--set auth.rootPassword=$(MINIO_SECRET_KEY)
	helm upgrade --install litellm k8s/helm/litellm -n $(NAMESPACE) \
		-f k8s/helm/litellm/values.yaml \
		-f k8s/helm/litellm/values.$(ENV).yaml
	helm upgrade --install langfuse k8s/helm/langfuse -n $(NAMESPACE) \
		-f k8s/helm/langfuse/values.yaml \
		-f k8s/helm/langfuse/values.$(ENV).yaml
	$(MAKE) deploy-observability
	@echo "✓ Infrastructure deployed"

deploy-observability: ## Deploy LGTM observability stack
	@echo "→ Deploying observability stack..."
	helm upgrade --install alloy grafana/alloy -n $(NAMESPACE) \
		-f k8s/helm/observability/alloy/values.yaml \
		-f k8s/helm/observability/alloy/values.$(ENV).yaml
	helm upgrade --install tempo grafana/tempo -n $(NAMESPACE) \
		-f k8s/helm/observability/tempo/values.yaml
	helm upgrade --install loki grafana/loki -n $(NAMESPACE) \
		-f k8s/helm/observability/loki/values.yaml
	helm upgrade --install prometheus prometheus-community/prometheus -n $(NAMESPACE) \
		-f k8s/helm/observability/prometheus/values.yaml
	helm upgrade --install grafana grafana/grafana -n $(NAMESPACE) \
		-f k8s/helm/observability/grafana/values.yaml
	@echo "✓ Observability stack deployed"

deploy: ## Deploy all JobRadar services
	@echo "→ Deploying services (ENV=$(ENV))..."
	@for svc in $(SERVICES); do \
		echo "  deploying $$svc..."; \
		kubectl apply -f k8s/manifests/$$svc/ -n $(NAMESPACE); \
	done
	kubectl apply -f k8s/manifests/frontend/ -n $(NAMESPACE)
	@echo "✓ All services deployed"

deploy-service: ## Deploy a single service (make deploy-service SVC=fetcher)
	@[ -n "$(SVC)" ] || (echo "✗ SVC is required. Usage: make deploy-service SVC=fetcher" && exit 1)
	@echo "→ Deploying $(SVC)..."
	kubectl apply -f k8s/manifests/$(SVC)/ -n $(NAMESPACE)
	@echo "✓ $(SVC) deployed"

undeploy: ## Remove all JobRadar services (keeps infrastructure)
	@echo "→ Removing services..."
	@for svc in $(SERVICES); do \
		kubectl delete -f k8s/manifests/$$svc/ -n $(NAMESPACE) --ignore-not-found; \
	done
	kubectl delete -f k8s/manifests/frontend/ -n $(NAMESPACE) --ignore-not-found
	@echo "✓ Services removed"

rollout-restart: ## Restart all deployments (picks up new images)
	@echo "→ Restarting deployments..."
	@for svc in $(SERVICES); do \
		kubectl rollout restart deployment/$$svc -n $(NAMESPACE); \
	done
	kubectl rollout restart deployment/frontend -n $(NAMESPACE)
	@echo "✓ Rollout triggered"

rollout-restart-service: ## Restart a single deployment (make rollout-restart-service SVC=fetcher)
	@[ -n "$(SVC)" ] || (echo "✗ SVC is required. Usage: make rollout-restart-service SVC=fetcher" && exit 1)
	kubectl rollout restart deployment/$(SVC) -n $(NAMESPACE)

# ------------------------------------------------------------------------------
# Development helpers
# ------------------------------------------------------------------------------
port-forward: ## Forward all service ports to localhost
	@echo "→ Forwarding ports (background)..."
	kubectl port-forward svc/api-gateway  8080:8080 -n $(NAMESPACE) &
	kubectl port-forward svc/mcp-server   8090:8090 -n $(NAMESPACE) &
	kubectl port-forward svc/a2a-agent    8091:8091 -n $(NAMESPACE) &
	kubectl port-forward svc/frontend     5173:80   -n $(NAMESPACE) &
	kubectl port-forward svc/grafana      3100:3000 -n $(NAMESPACE) &
	kubectl port-forward svc/langfuse     3101:3000 -n $(NAMESPACE) &
	kubectl port-forward svc/postgresql   5432:5432 -n $(NAMESPACE) &
	kubectl port-forward svc/kafka        9092:9092 -n $(NAMESPACE) &
	kubectl port-forward svc/valkey       6379:6379 -n $(NAMESPACE) &
	kubectl port-forward svc/minio        9000:9000 -n $(NAMESPACE) &
	kubectl port-forward svc/litellm      4000:4000 -n $(NAMESPACE) &
	@echo "✓ Ports forwarded — use 'make port-forward-stop' to stop"

port-forward-stop: ## Stop all port-forwards
	pkill -f "kubectl port-forward" || true
	@echo "✓ Port forwards stopped"

logs: ## Tail logs for a service (make logs SVC=fetcher)
	@[ -n "$(SVC)" ] || (echo "✗ SVC is required. Usage: make logs SVC=fetcher" && exit 1)
	kubectl logs -f deployment/$(SVC) -n $(NAMESPACE)

# ------------------------------------------------------------------------------
# Testing
# ------------------------------------------------------------------------------
test: test-unit test-integration ## Run all tests

test-unit: ## Run unit tests
	@echo "→ Running unit tests..."
	go test -v -race -count=1 ./services/.../internal/... ./services/.../domain/...
	@echo "✓ Unit tests passed"

test-integration: ## Run integration tests (requires running cluster)
	@echo "→ Running integration tests..."
	go test -v -race -count=1 -tags=integration ./services/.../...
	@echo "✓ Integration tests passed"

test-coverage: ## Run tests with coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

test-service: ## Run tests for a single service (make test-service SVC=fetcher)
	@[ -n "$(SVC)" ] || (echo "✗ SVC is required. Usage: make test-service SVC=fetcher" && exit 1)
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

# ------------------------------------------------------------------------------
# Full workflows
# ------------------------------------------------------------------------------
dev: cluster-up deploy-infra images-build images-load-kind deploy port-forward ## Full local dev setup
	@echo ""
	@echo "✓ JobRadar running locally"
	@echo ""
	@echo "  Frontend:    http://localhost:5173"
	@echo "  API:         http://localhost:8080"
	@echo "  MCP:         http://localhost:8090/mcp"
	@echo "  Grafana:     http://localhost:3100"
	@echo "  Langfuse:    http://localhost:3101"
	@echo ""

redeploy-service: images-build-service images-load-kind deploy-service rollout-restart-service ## Rebuild and redeploy a single service (make redeploy-service SVC=fetcher)
	@echo "✓ $(SVC) redeployed"