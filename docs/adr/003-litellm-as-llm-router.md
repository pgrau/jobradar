# ADR-003: LiteLLM as model-agnostic LLM router

- **Status:** Accepted
- **Date:** 2026-03-21
- **Author:** Pau Ferran Grau

---

## Context

JobRadar requires LLM inference for three operations: generating embeddings (embedder service), scoring job offers against a user CV (llm-service), and summarizing offers for the frontend. These operations run in different environments — Ollama locally during development, Google Gemini in production on Hetzner.

Without a routing layer, each service would need environment-specific LLM client code, credential management per model, and manual switching between local and cloud inference. Adding or swapping a model would require code changes across multiple services.

A routing layer solves this cleanly — one endpoint, one client, environment-specific backends defined in configuration.

---

## Decision

Use **LiteLLM proxy** as a model-agnostic router in front of all LLM backends.

---

## Rationale

### Single OpenAI-compatible endpoint

LiteLLM exposes a single `/v1/chat/completions` and `/v1/embeddings` endpoint compatible with the OpenAI API spec. All internal Go services call one endpoint regardless of which model is running behind it. Switching from Ollama to Gemini — or adding a third model — requires zero code changes in the services.

### Environment isolation without code changes

Local development uses Ollama (free, no API key, runs on the M2 MacBook Pro). Production on Hetzner uses Google Gemini. LiteLLM's `values.local.yaml` and `values.hetzner.yaml` Helm values define which backend is active per environment. The Go services are identical in both environments.

### Cost tracking and budget enforcement

LiteLLM provides per-model cost tracking out of the box. For a multi-user platform like JobRadar, where scoring runs per offer per user, cost visibility is critical. LiteLLM can enforce per-key spending limits — preventing a single user or a runaway scorer from exhausting the Gemini quota.

### Langfuse integration

LiteLLM natively integrates with Langfuse via a callback. Every LLM call — prompt, response, model, latency, token count, cost — is logged to Langfuse automatically without instrumentation in the Go services. This provides full LLM observability in Grafana alongside application traces from Tempo.

### Model fallback

LiteLLM supports fallback chains: if Gemini is unavailable or rate-limited, it can automatically retry with a secondary model. For a production deployment this is a reliability improvement with no additional code.

---

## Consequences

**Positive**
- All Go services are model-agnostic — one HTTP client, one endpoint
- Environment switching (local ↔ production) via Helm values only
- Cost tracking and budget limits per API key out of the box
- Langfuse observability with zero service-level instrumentation
- Model fallback without code changes

**Negative**
- Additional network hop — every LLM call goes through the proxy
- LiteLLM proxy is an additional service to operate and monitor
- Proxy becomes a single point of failure — mitigated by K8s readiness probes and replica count

---

## Alternatives considered

| Option | Reason rejected |
|---|---|
| Direct Ollama/Gemini SDK per service | Environment-specific code in every service, no central observability |
| OpenRouter | External dependency, less control over routing logic and cost |
| Custom Go proxy | Significant implementation effort, reinventing what LiteLLM already provides |
| Portkey | Similar to LiteLLM but less mature Go ecosystem integration |

---

## Configuration

LiteLLM is deployed via Helm with environment-specific values:

```yaml
# values.local.yaml
model_list:
  - model_name: default
    litellm_params:
      model: ollama/llama3.2
      api_base: http://ollama:11434

  - model_name: embeddings
    litellm_params:
      model: ollama/mxbai-embed-large
      api_base: http://ollama:11434
```

```yaml
# values.hetzner.yaml
model_list:
  - model_name: default
    litellm_params:
      model: gemini/gemini-2.0-flash
      api_key: os.environ/GEMINI_API_KEY

  - model_name: embeddings
    litellm_params:
      model: gemini/gemini-embedding-001
      api_key: os.environ/GEMINI_API_KEY
      api_base: https://generativelanguage.googleapis.com
      optional_params:
        output_dimensionality: 1024
```

Go services call LiteLLM via the OpenAI-compatible endpoint:

```go
client := openai.NewClient(
    option.WithBaseURL("http://litellm:4000"),
    option.WithAPIKey("sk-jobradar"),
)
```