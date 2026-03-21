# ADR-008: MCP server for LLM client tooling

- **Status:** Accepted
- **Date:** 2026-03-21
- **Author:** Pau Ferran Grau

---

## Context

JobRadar's job market intelligence capabilities are useful not only to human users via the frontend, but also to developers using LLM-powered coding assistants like Claude Code or Cursor. A developer actively job searching could query JobRadar directly from their editor — "find me Staff Backend Engineer roles in Europe with Go and K8s" — without leaving their workflow.

The Model Context Protocol (MCP) by Anthropic is the emerging standard for exposing tools and data sources to LLM clients. It defines how a server exposes tools, how an LLM client discovers and calls them, and how results are returned.

This ADR covers the decision to implement an MCP server and how it integrates into the JobRadar architecture. The relationship with A2A (ADR-007) is also addressed.

---

## Decision

Implement a dedicated **`mcp-server`** Go service exposing JobRadar capabilities via MCP over streamable HTTP.

---

## Rationale

### MCP as the standard for LLM tool access

MCP has become the de facto standard for LLM client tool integration — supported natively by Claude Code, Cursor, Windsurf, and others. Implementing MCP makes JobRadar accessible from any MCP-compatible client without custom integration work per client.

The alternative — a custom REST API that developers query manually — requires the developer to context-switch out of their editor, formulate a query, interpret raw JSON, and paste results back. MCP eliminates this friction entirely.

### Streamable HTTP transport

MCP supports multiple transports: stdio (local process) and streamable HTTP (network). JobRadar uses **streamable HTTP** because:

- The MCP server runs inside Kubernetes — not as a local process on the developer's machine
- Streamable HTTP allows the server to stream tool results progressively — useful for queries that return many offers
- Multiple MCP clients can connect simultaneously to the same server instance

```json
{
  "mcpServers": {
    "jobradar": {
      "type": "streamableHttp",
      "url": "http://localhost:8090/mcp"
    }
  }
}
```

### Dedicated service — protocol isolation

The MCP server is a dedicated service rather than an endpoint on the `api-gateway` for the same reason as A2A — protocol separation. The `api-gateway` serves REST to the frontend (optimized for browser clients), the `mcp-server` serves MCP to LLM clients (optimized for tool use patterns).

Both call the same internal gRPC services without modification.

### Tool design — useful for a developer job searching

The exposed tools are designed around real developer workflows during a job search:

```
"Find me Staff Backend Engineer roles in Europe posted this week"
  → search_offers(query="Staff Backend Engineer", location="Europe", days=7)

"How does this offer compare to my profile?"
  → score_offer(url="https://...")

"What skills should I add to my CV to get more matches?"
  → get_trending_skills(role="Staff Backend Engineer", region="Europe")
```

Each tool maps to a real question a developer asks during a job search — not generic CRUD operations.

### MCP vs A2A — different callers, different interaction models

As established in ADR-007:

- **MCP** — a developer's LLM assistant (Claude Code) calling JobRadar tools synchronously during a coding or research session
- **A2A** — an autonomous agent delegating a long-running analysis task to JobRadar as a peer

A single `mcp-server` request returns a result immediately. An A2A task streams progress over time. The protocols are complementary — both reuse the same internal gRPC services.

---

## Consequences

**Positive**
- JobRadar accessible from Claude Code, Cursor, and any MCP-compatible client
- Streamable HTTP allows progressive results and multiple concurrent clients
- Tool design mirrors real developer job search workflows
- Internal gRPC services reused without modification
- Protocol isolated from REST (`api-gateway`) and A2A (`a2a-agent`)

**Negative**
- MCP ecosystem still maturing — spec may evolve
- Additional service to deploy and monitor
- Streamable HTTP requires proper connection handling and timeout management in Go

---

## Alternatives considered

| Option | Reason rejected |
|---|---|
| MCP endpoint inside api-gateway | Mixes REST and MCP protocol concerns — different optimization targets |
| stdio transport | Requires MCP server as a local process — not suitable for K8s deployment |
| Custom REST API for LLM clients | No standard — requires custom integration per LLM client, no discoverability |
| OpenAI function calling format | Proprietary, not portable across LLM clients |

---

## Exposed MCP tools

| Tool | Input | Description |
|---|---|---|
| `search_offers` | `query`, `location?`, `days?`, `min_score?` | Semantic search over ingested offer history |
| `score_offer` | `url` or `text` | Score a job offer against the authenticated user's CV |
| `get_trending_skills` | `role?`, `region?`, `period?` | Skills trending in job descriptions this week/month |
| `get_top_matches` | `limit?`, `min_score?` | Highest-scoring unreviewed offers for the current user |
| `get_company_intel` | `company` | Aggregated hiring data for a specific company |
| `get_my_profile` | — | Current user profile summary and CV embedding status |