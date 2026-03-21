# ADR-007: Google A2A protocol for AI agent interoperability

- **Status:** Accepted
- **Date:** 2026-03-21
- **Author:** Pau Ferran Grau

---

## Context

JobRadar exposes job market intelligence capabilities (offer scoring, semantic search, trend analysis) that are useful not only to human users via the frontend, but also to AI agents operating autonomously. As multi-agent architectures become common, enabling JobRadar to participate as a node in a larger agent ecosystem requires a standardized communication protocol.

Two complementary protocols exist for this use case: **MCP** (Model Context Protocol by Anthropic) and **A2A** (Agent-to-Agent by Google). They solve different problems and are not mutually exclusive — MCP is covered in ADR-008.

The question is whether to implement A2A support, and if so, how to integrate it into the existing architecture.

---

## Decision

Implement an **A2A agent** as a dedicated `a2a-agent` Go service exposing JobRadar capabilities via the Google A2A protocol.

---

## Rationale

### A2A vs MCP — complementary, not competing

A2A and MCP solve different problems and target different interaction patterns:

| | MCP | A2A |
|---|---|---|
| Primary use case | LLM client uses tools to access data | Agent delegates tasks to another agent |
| Interaction model | Tool call / response | Task lifecycle (submit, stream, complete) |
| Caller | LLM with tool use (Claude Code, Cursor) | Another AI agent (Google ADK, custom) |
| Protocol | JSON-RPC over HTTP/stdio | HTTP + JSON with task state machine |
| JobRadar role | Tool provider | Agent peer |

MCP is for a developer asking Claude Code "find me Go jobs in Barcelona" via a tool. A2A is for an orchestrator agent delegating "analyze the job market for backend engineers in Europe this week" as a long-running task.

### Dedicated service — clean separation of concerns

A2A introduces a task lifecycle state machine (submitted → working → completed/failed) with streaming updates. This logic belongs in a dedicated service, not mixed into the `api-gateway` (which serves synchronous REST to the frontend) or the `mcp-server` (which serves tool calls to LLM clients).

Each service has a single protocol responsibility:
- `api-gateway` → REST for the frontend
- `mcp-server` → MCP for LLM clients
- `a2a-agent` → A2A for agent-to-agent communication

All three call the same internal gRPC services (`llm-service`, `rag-service`, `embedder`).

### Task streaming via A2A

A2A supports streaming task updates — the agent emits intermediate results as it works. For a scoring task that involves multiple gRPC calls (embed → search similar → score → summarize), streaming progress back to the calling agent is a better UX than a single blocking response.

```
A2A task: "score these 50 offers against profile X"
  → submitted
  → working: scored 10/50...
  → working: scored 30/50...
  → completed: results + summary
```

### Agent Card — discoverability

A2A defines an Agent Card — a JSON document at `/.well-known/agent.json` that describes the agent's capabilities, skills, and authentication requirements. Any A2A-compatible orchestrator can discover and invoke JobRadar's capabilities without prior configuration.

```json
{
  "name": "JobRadar",
  "description": "Job market intelligence — offer scoring, semantic search, trend analysis",
  "skills": [
    { "id": "score_offer", "description": "Score a job offer against a user profile" },
    { "id": "search_market", "description": "Semantic search over the job offer corpus" },
    { "id": "get_trends", "description": "Market skill trends for a given role or stack" }
  ]
}
```

---

## Consequences

**Positive**
- JobRadar participates as a peer in multi-agent architectures
- Clean protocol separation — A2A, MCP, and REST in dedicated services
- Task streaming provides better experience for long-running operations
- Agent Card enables automatic discoverability by A2A orchestrators
- Internal gRPC services reused without modification

**Negative**
- A2A is a relatively new protocol (Google, 2025) — ecosystem still maturing
- Additional service to deploy, monitor, and maintain
- Task state management adds complexity compared to simple request/response

---

## Alternatives considered

| Option | Reason rejected |
|---|---|
| A2A inside api-gateway | Mixes REST and A2A protocol concerns in one service |
| A2A inside mcp-server | MCP and A2A have different interaction models — conflating them adds complexity |
| OpenAI Assistants API pattern | Proprietary, not interoperable with non-OpenAI agents |
| Custom agent protocol | Reinvents the wheel — A2A is an open standard with growing adoption |

---

## Exposed A2A skills

| Skill ID | Description | Internal calls |
|---|---|---|
| `score_offer` | Score a job offer against a profile | embedder → rag-service → llm-service |
| `search_market` | Semantic search over offer corpus | embedder → rag-service |
| `get_trends` | Skill trends for a role or stack | rag-service → llm-service |