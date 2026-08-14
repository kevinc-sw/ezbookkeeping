# ezBookkeeping Architecture

Lightweight, self-hosted personal finance app: **Go + Gin** backend and **Vue 3** dual SPA (desktop / mobile), packaged as a single binary / Docker image.

This folder documents the **as-built** design. No application code was changed.

## Documents

| Doc | Focus |
| --- | --- |
| [System overview](./system-overview.md) | Runtime topology, boot, deployment |
| [Backend](./backend.md) | Layers, request lifecycle, packages |
| [Frontend](./frontend.md) | Dual SPA, stores, API client, PWA |
| [Domain model](./domain-model.md) | Entities and relationships |
| [Integrations](./integrations.md) | Auth, LLM, MCP, FX, storage, import |

## System context

```mermaid
flowchart LR
    User["User<br/>Browser / PWA"]
    Agent["AI agent<br/>MCP client"]
    EZ["ezBookkeeping<br/>Go + Gin + Vue SPAs"]

    DB[(SQLite / MySQL / PostgreSQL)]
    OBJ["Object storage<br/>Local / MinIO / WebDAV"]
    OAuth["OAuth / OIDC"]
    LLM["LLM providers"]
    FX["FX data sources"]
    SMTP["SMTP"]
    Maps["Map tile providers"]

    User -->|HTTPS JSON| EZ
    Agent -->|JSON-RPC /mcp| EZ
    EZ --> DB
    EZ --> OBJ
    EZ --> OAuth
    EZ --> LLM
    EZ --> FX
    EZ --> SMTP
    EZ --> Maps
```

## High-level stack

```mermaid
flowchart TB
    subgraph Clients
        Desktop["Desktop SPA<br/>Vue 3 + Vuetify"]
        Mobile["Mobile SPA<br/>Vue 3 + Framework7"]
        MCPClient["MCP / AI agents"]
        CLI["CLI subcommands"]
    end

    subgraph Binary["ezBookkeeping process"]
        Gin["Gin HTTP"]
        API["pkg/api"]
        Svc["pkg/services"]
        Cron["pkg/cron"]
        MCP["pkg/mcp"]
    end

    subgraph Persistence
        DB[(DB via XORM)]
        Store["Object storage"]
    end

    Desktop --> Gin
    Mobile --> Gin
    MCPClient --> Gin
    CLI --> Svc
    Gin --> API
    Gin --> MCP
    API --> Svc
    MCP --> Svc
    Cron --> Svc
    Svc --> DB
    Svc --> Store
```

## Design principles (observed)

1. **Single deployable** — static frontend assets + API + cron + MCP in one process.
2. **Strict layering** — `api` → `services` → `datastore` / `storage`; UI never talks to DB.
3. **Container singletons** — config-driven providers (LLM, OAuth, FX, storage, avatars).
4. **Feature flags** — routes and subsystems gated by config without recompile.
5. **Dual UI, shared core** — desktop/mobile share stores, models, and `*Base.ts` logic.
6. **Revocable auth** — JWT validated against per-token DB secrets (`TokenRecord`).
