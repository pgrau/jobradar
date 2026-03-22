# JobRadar

> Real-time job market intelligence platform powered by AI — ingests job offers from multiple sources, scores them against your profile, and surfaces market trends.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go](https://img.shields.io/badge/Go-1.26-blue.svg)](https://golang.org)
[![K8s](https://img.shields.io/badge/Kubernetes-kind-326CE5.svg)](https://kind.sigs.k8s.io)

## Overview

JobRadar ingests job offers from LinkedIn, RemoteOK, Remotive, WeWorkRemotely, InfoJobs and direct company APIs, processes them through an AI analysis pipeline, and delivers personalized insights via a React dashboard and smart alerts.

**Repository:** [github.com/pgrau/jobradar](https://github.com/pgrau/jobradar)

- **Multi-user platform** — register, upload your CV, get personalized matches
- **Real-time ingestion** with Kafka — new offers processed in seconds
- **AI-powered CV scoring** — semantic match between your profile and each offer
- **Market trend analysis** — which skills are rising, which companies are hiring, salary ranges
- **Personalized alerts** — notified only when a role genuinely matches your profile
- **gRPC-based internal communication** between Go microservices
- **RAG pipeline** using PostgreSQL 18 + pgvector — semantic search over offer history
- **MCP server** — use JobRadar as a toolset from Claude Code or Cursor
- **A2A protocol** — expose JobRadar as an interoperable AI agent
- **Full observability** with Grafana LGTM stack (Alloy, Tempo, Loki, Prometheus) + Langfuse
- **CV storage** with MinIO (S3-compatible, self-hosted)
- **Infrastructure as Code** with Terraform (Hetzner — production deployment)

---

## Architecture

```
  LinkedIn · RemoteOK · Remotive
  WWR · InfoJobs · Company APIs
            │
            ▼
  ┌─────────────────────────────────────────────────────────────┐
  │                    Kubernetes Cluster                        │
  │                                                             │
  │  ┌─────────────┐         ┌──────────────────────────────┐  │
  │  │  fetcher    │────────►│           Kafka              │  │
  │  │  (Go)       │         │  topics: raw-offers          │  │
  │  └─────────────┘         │          scored-offers       │  │
  │                          │          alerts              │  │
  │                          └──────────────┬───────────────┘  │
  │                                         │                   │
  │                                         ▼                   │
  │                          ┌──────────────────────────────┐  │
  │                          │       scorer (Go)            │  │
  │                          │  Kafka consumer + gRPC client│  │
  │                          └──────────────┬───────────────┘  │
  │                                         │ gRPC              │
  │                   ┌─────────────────────┼──────────────┐   │
  │                   │                     │              │   │
  │                   ▼                     ▼              ▼   │
  │          ┌─────────────┐      ┌──────────────┐  ┌─────────┐│
  │          │ llm-service │      │  rag-service │  │embedder ││
  │          │ (Go/gRPC)   │      │  (Go/gRPC)   │  │(Go/gRPC)││
  │          └──────┬──────┘      └──────┬───────┘  └────┬────┘│
  │                 │                    │               │     │
  │                 ▼                    ▼               ▼     │
  │          ┌─────────────┐    ┌─────────────────────────┐   │
  │          │   LiteLLM   │    │   PostgreSQL 18          │   │
  │          │   Proxy     │    │   + pgvector             │   │
  │          └──────┬──────┘    └─────────────────────────┘   │
  │                 │                                           │
  │          ┌──────┴───────┐                                  │
  │          ▼              ▼                                   │
  │      ┌────────┐   ┌─────────┐                              │
  │      │ Ollama │   │ Gemini  │                              │
  │      │(local) │   │ (cloud) │                              │
  │      └────────┘   └─────────┘                              │
  │                                                             │
  │  ┌──────────────────────────────────────────────────────┐  │
  │  │              Observability · LGTM stack              │  │
  │  │  Grafana Alloy → Tempo (traces)                      │  │
  │  │                → Loki (logs)                         │  │
  │  │                → Prometheus (metrics)                │  │
  │  │  Langfuse (LLM traces) ──► Tempo                     │  │
  │  │  Grafana (unified dashboards)                        │  │
  │  └──────────────────────────────────────────────────────┘  │
  │                                                             │
  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
  │  │  api-gateway │  │  mcp-server  │  │   a2a-agent      │  │
  │  │  (Go/REST)   │  │  (Go/MCP)    │  │   (Go/A2A)       │  │
  │  └──────────────┘  └──────────────┘  └──────────────────┘  │
  │                                                             │
  │  ┌──────────────────────────────────────────────────────┐  │
  │  │            Valkey (cache + rate limiting)            │  │
  │  └──────────────────────────────────────────────────────┘  │
  └─────────────────────────────────────────────────────────────┘
            │
            ▼
  ┌─────────────────────┐
  │  Frontend           │
  │  React 19 + TS      │
  │  apps/frontend/     │
  └─────────────────────┘
```

### Services

| Service | Language | Protocol | Responsibility |
|---|---|---|---|
| `fetcher` | Go | Kafka producer | Fetches offers from LinkedIn, RemoteOK, Remotive, WWR, InfoJobs, company APIs |
| `auth` | Go | REST | JWT registration, login, refresh tokens, profile management |
| `scorer` | Go | Kafka consumer + gRPC client | Orchestrates AI scoring pipeline per offer |
| `llm-service` | Go | gRPC server | LLM inference via LiteLLM — scoring, summarization, trend analysis |
| `rag-service` | Go | gRPC server | pgvector semantic search over offer history |
| `embedder` | Go | gRPC server | Vector embeddings via LiteLLM |
| `api-gateway` | Go | REST (public) + gRPC client | Public query API — JWT middleware, offers, scores, trends, alerts |
| `mcp-server` | Go | MCP over streamable HTTP | Exposes JobRadar tools to LLM clients |
| `a2a-agent` | Go | A2A protocol | Exposes JobRadar as an interoperable AI agent |

### Infrastructure

| Component | Purpose |
|---|---|
| Kafka | Async event stream — raw offers, scored offers, alerts |
| PostgreSQL 18 + pgvector | Offer storage, embeddings, RAG pipeline |
| Valkey | LLM response cache, API rate limiting per user |
| MinIO | CV file storage (S3-compatible, self-hosted) |
| LiteLLM | LLM routing — Ollama (local) / Gemini (cloud) |
| Langfuse | LLM observability and tracing |
| Grafana Alloy | OTel collector — routes to Tempo, Loki, Prometheus |
| Grafana Tempo | Distributed tracing |
| Grafana Loki | Structured log aggregation |
| Prometheus | Metrics — services, Kafka, K8s |
| Grafana | Unified dashboards |
| Helm | Kubernetes package management |
| Terraform | Infrastructure as Code (Hetzner-ready) |

---

## User Workflow

```
1. Register & login
   └── JWT auth via api-gateway
   └── profile_id assigned — all data scoped to this user

2. Upload CV (PDF)
   └── Stored in MinIO
   └── Text extracted → embedder → pgvector
   └── Profile configured: skills, seniority, location, salary range, alert rules

3. Fetcher runs continuously
   └── Ingests offers from LinkedIn, RemoteOK, Remotive, WWR, InfoJobs
   └── Publishes to Kafka → raw-offers (with source metadata)

4. Scorer processes each offer per registered user
   └── Embeddings → pgvector similarity vs user CV
   └── LLM scoring (0-100) + reasoning per user profile
   └── Publishes to Kafka → scored-offers

5. Frontend dashboard
   └── Feed of offers ranked by personal score
   └── LLM reasoning visible per offer ("Strong match: Go + DDD. Gap: Terraform")
   └── Market trends — skills rising, companies hiring, salary ranges
   └── Real-time alerts via SSE when score ≥ configured threshold
```

**Data isolation:** every PostgreSQL query, Kafka message, Valkey key, and pgvector search is scoped by `profile_id`. Multi-tenancy is enforced at the data layer, not just the API layer.

---

## Core Features

### CV Scoring

Each ingested offer is semantically scored against your profile using embeddings + LLM reasoning:

```
offer text + your CV
    └──► embedder (vectors)
    └──► rag-service (similar past offers)
    └──► llm-service (score 0-100 + reasoning)
         └── "Strong match: Go + DDD + distributed systems.
              Gap: no Terraform experience mentioned."
```

Scores are stored in PostgreSQL and surfaced in the dashboard with full LLM reasoning.

### Market Trend Analysis

The `scorer` aggregates scored offers over time to detect:
- Skills trending up/down in job descriptions (Go, K8s, Rust, AI...)
- Companies actively hiring for your profile
- Salary range evolution by role and location
- Remote vs hybrid vs onsite ratio trends

### Personalized Alerts

Configurable alert rules evaluated after scoring:
- Score threshold (e.g. notify if score ≥ 80)
- New company hiring for your stack
- Salary above a threshold
- Role in a specific location or fully remote

Alerts published to Kafka → api-gateway → frontend (SSE) + optional webhook/email.

---

## MCP Server

JobRadar exposes an MCP server over streamable HTTP — use JobRadar as a toolset from Claude Code or Cursor while you work.

```
Claude Code / Cursor / any MCP client
    └──► mcp-server (Go · streamable HTTP)
              ├──► rag-service  (gRPC) — search_offers
              ├──► llm-service  (gRPC) — score_offer
              └──► api-gateway  (REST) — get_trending_skills
```

**Exposed MCP tools:**

| Tool | Description |
|---|---|
| `search_offers` | Semantic search over ingested offer history |
| `score_offer` | Score a job offer URL or text against your CV |
| `get_trending_skills` | Skills trending in the market this week/month |
| `get_top_matches` | Your highest-scoring unreviewed offers |
| `get_company_intel` | Aggregated data on a specific company |
| `get_my_profile` | Retrieve current user profile and CV summary |

**MCP client config:**

```json
{
  "mcpServers": {
    "jobradar": {
      "type": "streamableHttp",
      "url": "http://localhost:8090/mcp"
      // Production: https://jobradar.yourdomain.com/mcp
    }
  }
}
```

---

## A2A Agent

JobRadar exposes its capabilities via the Google Agent-to-Agent (A2A) protocol, enabling external AI agents to request job market intelligence in a standardized, interoperable way.

```
External A2A Agent
    └──► a2a-agent (Go · A2A endpoint)
              ├──► llm-service  (gRPC) — scoring, analysis
              ├──► rag-service  (gRPC) — semantic search
              └──► embedder     (gRPC) — similarity
```

**Exposed A2A skills:**

| Skill | Description |
|---|---|
| `score_offer` | Score a job offer against a given profile |
| `search_market` | Semantic search over the job offer corpus |
| `get_trends` | Market skill trends for a given role or stack |

---

## Observability

All services are instrumented with **OpenTelemetry SDK for Go**. Traces propagate automatically across gRPC boundaries via interceptors and across HTTP via middleware.

```go
// gRPC — automatic trace propagation
grpc.NewServer(
    grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
    grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
)

// HTTP api-gateway
handler = otelhttp.NewHandler(mux, "api-gateway")
```

**What you see in Grafana:**

| Signal | What it shows |
|---|---|
| Traces (Tempo) | End-to-end: HTTP → scorer → llm-service → pgvector |
| Logs (Loki) | Structured JSON logs correlated to traces with one click |
| Metrics (Prometheus) | Request rate, p50/p95/p99 latency, Kafka consumer lag |
| LLM traces (Langfuse → Tempo) | Token usage, model latency, score reasoning per offer |

---

## Secrets

Kubernetes secrets are **never committed** to the repository. Each secret has a `.yaml.example` template.

**Setup before first deploy:**

```bash
# LiteLLM — PostgreSQL credentials
cp k8s/manifests/litellm/secret.yaml.example k8s/manifests/litellm/secret.yaml
# edit with your values
kubectl apply -f k8s/manifests/litellm/secret.yaml -n jobradar
```

**All secret templates:**

| File | Secret name | Description |
|---|---|---|
| `k8s/manifests/litellm/secret.yaml.example` | `postgres` | LiteLLM PostgreSQL credentials |

> Secrets are gitignored via `k8s/**/*secret*.yaml`. Never commit real credentials.

---

## Development Workflow

This project follows **Spec-Driven Development** with Claude Code for AI-assisted implementation.

Architecture decisions and technical specifications are human-authored (see [ADRs](docs/adr/)). Claude Code assists with implementation within those constraints — scaffolding services, generating protobuf stubs, writing Helm values, and producing test boilerplate.

---

## Getting Started (Local)

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [kind](https://kind.sigs.k8s.io/) — `brew install kind`
- [kubectl](https://kubernetes.io/docs/tasks/tools/) — `brew install kubectl`
- [helm](https://helm.sh/) — `brew install helm`
- [skaffold](https://skaffold.dev/) — `brew install skaffold`
- [Ollama](https://ollama.ai/) — `brew install ollama`
- [grpcurl](https://github.com/fullstorydev/grpcurl) — `brew install grpcurl`
- `make`

### 1. Pull required Ollama models

```bash
ollama serve &
ollama pull mistral:7b
ollama pull mxbai-embed-large
```

### 2. Configure environment

```bash
cp .env.example .env
# edit .env with your values
```

### 3. Create cluster and deploy infrastructure

```bash
make cluster-up
make deploy-infra
```

### 4. Apply secrets

```bash
cp k8s/manifests/litellm/secret.yaml.example k8s/manifests/litellm/secret.yaml
kubectl apply -f k8s/manifests/litellm/secret.yaml -n jobradar
```

### 5. Deploy services

```bash
skaffold dev --profile local
```

### 6. Forward ports

```bash
make port-forward-infra
```

### 7. Access the services

#### Application

| Service | URL | Credentials |
|---|---|---|
| **Frontend** | http://localhost:5173 | — |
| **API Gateway** | http://localhost:8080 | — |
| **MCP Server** | http://localhost:8090/mcp | — |

#### AI & LLM

| Service | URL | Credentials | Description |
|---|---|---|---|
| **LiteLLM** | http://localhost:4000 | Bearer `sk-jobradar` | LLM proxy — routes to Ollama/Gemini |
| **LiteLLM UI** | http://localhost:4000/ui | Bearer `sk-jobradar` | Model management, usage stats |
| **LiteLLM Health** | http://localhost:4000/health | Bearer `sk-jobradar` | Model health status |

#### Observability

| Service | URL | Credentials | Description |
|---|---|---|---|
| **Langfuse** | http://localhost:3101 | create on first access | LLM traces — prompts, responses, latency, cost |
| **Grafana** | http://localhost:3100 | admin / admin | Unified dashboards — traces, logs, metrics |
| **Prometheus** | http://localhost:9090 | — | Raw metrics |
| **Tempo** | http://localhost:3200 | — | Distributed traces backend |
| **Loki** | http://localhost:3102 | — | Logs backend |

#### Infrastructure

| Service | URL / Host | Credentials | Description |
|---|---|---|---|
| **MinIO Console** | http://localhost:9001 | minioadmin / jobradar_local | S3 storage UI — CV files, Langfuse data |
| **MinIO API** | http://localhost:9000 | minioadmin / jobradar_local | S3 API endpoint |
| **PostgreSQL** | localhost:5432 | jobradar / jobradar_local | Primary database + pgvector |
| **Kafka** | localhost:9092 | — | Event streaming |
| **Valkey** | localhost:6379 | — | Cache + rate limiting |

### 8. First time — Langfuse setup

```bash
# 1. Open http://localhost:3101 and create admin user
# 2. Create a new project named "jobradar"
# 3. Go to Settings → API Keys and copy the keys to .env
LANGFUSE_PUBLIC_KEY=pk-...
LANGFUSE_SECRET_KEY=sk-...

# 4. Update LiteLLM with Langfuse keys
helm upgrade litellm oci://ghcr.io/berriai/litellm-helm \
  -n jobradar \
  -f k8s/helm/litellm/values.yaml
```

### Verify

```bash
kubectl get pods -n jobradar
```

---

## Day-to-day development

```bash
# Rebuild and redeploy a single service
make dev-service SVC=embedder

# Tail logs
kubectl logs -f deployment/embedder -n jobradar

# Run tests
make test
make test-service SVC=embedder

# Stop port-forwards
make port-forward-stop

# Tear down cluster
make cluster-down
```

---

## Environments

| Environment | K8s | LLM Backend | Infrastructure |
|---|---|---|---|
| `local` | kind | Ollama | Manual (kind) |
| `hetzner` | Hetzner K8s | Google Gemini | Terraform — production |

### Deploy to Hetzner

```bash
export HCLOUD_TOKEN=your_token
export GEMINI_API_KEY=your_key

cd infra/terraform/environments/hetzner
terraform init && terraform apply

make deploy ENV=hetzner
```

---

## Project Structure

```
jobradar/
├── proto/                          # Protobuf definitions (shared)
│   ├── llm/v1/llm.proto
│   ├── rag/v1/rag.proto
│   └── embedder/v1/embedder.proto
├── apps/
│   └── frontend/                   # React 19 + TypeScript + Vite
│       ├── src/
│       │   ├── components/
│       │   ├── pages/
│       │   └── hooks/
│       ├── Dockerfile
│       └── package.json
├── services/
│   ├── auth/                       # JWT auth + profile management
│   │   ├── cmd/main.go
│   │   ├── internal/
│   │   └── Dockerfile
│   ├── fetcher/                    # Kafka producer — job source crawlers
│   │   ├── cmd/main.go
│   │   ├── internal/
│   │   │   ├── sources/            # linkedin, remoteok, remotive, wwr, infojobs
│   │   │   └── publisher/
│   │   └── Dockerfile
│   ├── scorer/                     # Kafka consumer + gRPC client
│   ├── llm-service/                # gRPC server → LiteLLM
│   ├── rag-service/                # gRPC server → pgvector
│   ├── embedder/                   # gRPC server → LiteLLM
│   ├── api-gateway/                # REST public API
│   ├── mcp-server/                 # MCP over streamable HTTP
│   └── a2a-agent/                  # A2A protocol endpoint
├── k8s/
│   ├── helm/
│   │   ├── litellm/
│   │   │   ├── values.yaml
│   │   │   ├── values.local.yaml   # ollama endpoint
│   │   │   └── values.hetzner.yaml # gemini endpoint
│   │   ├── langfuse/
│   │   ├── kafka/
│   │   ├── postgresql/
│   │   ├── valkey/
│   │   ├── minio/
│   │   └── observability/
│   │       ├── alloy/
│   │       ├── tempo/
│   │       ├── loki/
│   │       ├── prometheus/
│   │       └── grafana/
│   └── manifests/
│       ├── auth/
│       ├── fetcher/
│       ├── scorer/
│       ├── llm-service/
│       ├── rag-service/
│       ├── embedder/
│       ├── api-gateway/
│       ├── mcp-server/
│       ├── a2a-agent/
│       └── frontend/
├── infra/
│   ├── local/
│   │   └── kind-cluster.yaml
│   └── terraform/
│       ├── modules/
│       │   ├── k8s-cluster/
│       │   ├── postgresql/
│       │   └── networking/
│       └── environments/
│           ├── hetzner/
│           └── digitalocean/
├── docs/
│   ├── adr/
│   │   ├── 001-kafka-over-rabbitmq.md
│   │   ├── 002-grpc-internal-communication.md
│   │   ├── 003-litellm-as-llm-router.md
│   │   ├── 004-kind-over-minikube.md
│   │   ├── 005-pgvector-for-rag.md
│   │   ├── 006-lgtm-observability-stack.md
│   │   ├── 007-a2a-agent-protocol.md
│   │   ├── 008-mcp-server.md
│   │   ├── 009-valkey-over-redis.md
│   │   └── 010-multi-user-design.md
│   └── ai-workflow.md
├── .github/
│   └── workflows/
│       ├── ci.yml          # build + test on every PR
│       └── release.yml     # build + push ghcr.io/pgrau/jobradar on merge to main
├── Makefile
└── README.md
```

---

## Architecture Decision Records

- **[ADR-001](docs/adr/001-kafka-over-rabbitmq.md)** — Kafka over RabbitMQ for the ingestion layer
- **[ADR-002](docs/adr/002-grpc-internal-communication.md)** — gRPC for internal service communication
- **[ADR-003](docs/adr/003-litellm-as-llm-router.md)** — LiteLLM as model-agnostic LLM router
- **[ADR-004](docs/adr/004-kind-over-minikube.md)** — kind over minikube for local K8s
- **[ADR-005](docs/adr/005-pgvector-for-rag.md)** — pgvector for RAG over dedicated vector DB
- **[ADR-006](docs/adr/006-lgtm-observability-stack.md)** — Grafana LGTM stack over standalone solutions
- **[ADR-007](docs/adr/007-a2a-agent-protocol.md)** — A2A protocol for AI agent interoperability
- **[ADR-008](docs/adr/008-mcp-server.md)** — MCP server for LLM client tooling
- **[ADR-009](docs/adr/009-valkey-over-redis.md)** — Valkey over Redis (license + governance)
- **[ADR-010](docs/adr/010-multi-user-design.md)** — Multi-user design with profile_id isolation

---

## Tech Stack

| Category | Technology |
|---|---|
| Language | Go 1.26 |
| Communication (internal) | gRPC + Protocol Buffers |
| Communication (public) | REST (gRPC-gateway) |
| Messaging | Apache Kafka |
| Database | PostgreSQL 18 + pgvector |
| Cache | Valkey |
| File Storage | MinIO (S3-compatible) |
| LLM Router | LiteLLM |
| LLM Backends | Ollama (local) · Google Gemini (cloud) |
| LLM Observability | Langfuse |
| Tracing | Grafana Tempo (via OTel + Grafana Alloy) |
| Logging | Grafana Loki |
| Metrics | Prometheus |
| Dashboards | Grafana |
| MCP | Model Context Protocol over streamable HTTP |
| Agent Protocol | Google A2A |
| Frontend | React 19 + TypeScript + Vite |
| Container Orchestration | Kubernetes (kind / Hetzner) |
| Package Management | Helm |
| Infrastructure as Code | Terraform |
| CI/CD | GitHub Actions (ghcr.io/pgrau/jobradar) |

---

## Scope & Roadmap

### v1

| Feature                                                           | Status    |
|-------------------------------------------------------------------|-----------|
| Multi-user with JWT auth                                          | 🔲 Pending |
| CV upload (PDF) + automatic embedding                             | 🔲 Pending |
| Job offer ingestion — LinkedIn, RemoteOK, Remotive, WWR, InfoJobs | 🔲 Pending |
| AI scoring per user profile                                       | 🔲 Pending |
| Market trend analysis                                             | 🔲 Pending |
| Real-time alerts via SSE                                          | 🔲 Pending |
| MCP server                                                        | 🔲 Pending |
| A2A agent                                                         | 🔲 Pending |
| Kubernetes deployment (local)                                     | ✅ Done    |
| Kubernetes deployment (hertz)                                     | 🔲 Pending |
| React frontend — job feed + search + trends                       | 🔲 Pending |
| Full observability (LGTM + Langfuse)                              | 🔲 Pending |

### v2

| Feature | Status |
|---|---|
| OAuth2 (Google login) | 💡 Planned |
| MongoDB for denormalized job feed | 💡 Planned |
| Mobile notifications (push or Telegram) | 💡 Planned |
| Company intel aggregation | 💡 Planned |
| Open source release | 💡 Planned |

> Legend: 🔲 Pending · 🚧 In progress · ✅ Done · 💡 Planned

---

## License

MIT — see [LICENSE](LICENSE) for details.

---

## Author

**Pau Ferran Grau** — Tech Lead · Staff Backend Engineer
Barcelona, Spain · [linkedin.com/in/pauferrangrau](https://linkedin.com/in/pauferrangrau) · [github.com/pgrau](https://github.com/pgrau)

> 20 years building distributed systems. Focused on AI-augmented engineering, event-driven architecture, and high-performance backend platforms.