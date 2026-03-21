# ADR-004: kind over minikube for local Kubernetes

- **Status:** Accepted
- **Date:** 2026-03-21
- **Author:** Pau Ferran Grau

---

## Context

JobRadar runs on Kubernetes. A local K8s environment is needed for development and testing before deploying to Hetzner. The two most common options for local K8s are **kind** (Kubernetes in Docker) and **minikube**.

The local environment needs to support a multi-node cluster (control plane + 2 workers), run all JobRadar services and infrastructure (Kafka, PostgreSQL 18, Valkey, LiteLLM, Langfuse, LGTM stack), and be reproducible across machines and CI pipelines.

---

## Decision

Use **kind** for local Kubernetes development.

---

## Rationale

### Cluster definition as code

kind defines the entire cluster topology in a single YAML file committed to the repo:

```yaml
# infra/local/kind-cluster.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: worker
  - role: worker
```

Any developer clones the repo and runs `make cluster-up` — the cluster is identical every time. minikube clusters are configured via CLI flags, which are harder to version and share.

### CI/CD compatibility

kind runs inside Docker containers, making it fully compatible with GitHub Actions runners without any additional setup. The same `kind-cluster.yaml` used locally spins up an identical cluster in CI for integration tests. minikube requires a VM driver or additional configuration to run in CI environments.

### Closer to production

kind nodes are Docker containers running real Kubernetes components — the same binaries that run on Hetzner. minikube introduces abstractions (VM driver, addons system) that can mask issues that only appear in production. What works in kind is more likely to work on Hetzner.

### Resource efficiency on M2 MacBook Pro

kind runs K8s inside existing Docker containers — no separate VM overhead. On an M2 MacBook Pro with 16GB RAM running a full stack (8 Go services + Kafka + PostgreSQL + Valkey + LiteLLM + Langfuse + LGTM), resource efficiency matters. minikube spins up a VM (even with the Docker driver) with additional overhead.

### Multi-node support

kind supports multi-node clusters natively via the YAML definition. minikube added multi-node support later and it remains less stable. JobRadar uses a 3-node cluster (1 control plane + 2 workers) to test pod scheduling and service distribution realistically.

---

## Consequences

**Positive**
- Cluster topology versioned in `infra/local/kind-cluster.yaml`
- Identical cluster in local development and GitHub Actions CI
- No VM overhead — runs inside Docker on M2
- Multi-node cluster with simple YAML configuration
- `kind load docker-image` for loading local images without a registry

**Negative**
- No built-in dashboard — requires manual `kubectl proxy` or installing Kubernetes Dashboard via Helm
- No built-in addons system — everything installed via Helm (which is the correct approach anyway)
- `kind load docker-image` required for every local image build — mitigated by `make images-load-kind` target

---

## Alternatives considered

| Option | Reason rejected |
|---|---|
| minikube | VM overhead, CLI-driven config harder to version, less CI-compatible |
| k3s / k3d | Lightweight but uses a non-standard K8s distribution — less representative of production |
| Docker Compose | No service discovery, no health checks, no rolling deployments — does not reflect production operational requirements for 8+ services |
| MicroK8s | Linux-only, not suitable for M2 MacBook Pro development |