# Integrations

Config-driven **provider** interfaces. Operators pick implementations without code changes.

## Auth integrations

```mermaid
flowchart TB
    subgraph Local
        Pass["Username / password"]
        TOTP["2FA TOTP"]
        Lock["Client app lock<br/>PIN / WebAuthn"]
    end

    subgraph External
        OIDC["Generic OIDC"]
        GH["GitHub"]
        Gitea["Gitea"]
        NC["Nextcloud"]
    end

    Pass --> Tokens["TokenService"]
    TOTP --> Tokens
    OIDC --> OAuth["pkg/auth/oauth2"]
    GH --> OAuth
    Gitea --> OAuth
    NC --> OAuth
    OAuth --> Tokens
    Tokens --> API["/api/v1 JWT"]
    Tokens --> MCP["/mcp JWT"]
    Lock -.-> UI["Frontend only"]
```

OAuth flow uses `/oauth2/login` + `/oauth2/callback` and short-lived callback tokens; PKCE supported where applicable.

## LLM

```mermaid
flowchart LR
    API["/api/v1/llm/…"] --> LLMAPI["large_language_models API"]
    LLMAPI --> Cont["LargeLanguageModelProvider container"]
    Cont --> Text["Text recognition provider"]
    Cont --> Img["Receipt image recognition provider"]

    Text --> P1["OpenAI / compatible"]
    Text --> P2["Anthropic / compatible"]
    Text --> P3["Ollama / LM Studio"]
    Text --> P4["Google AI / OpenRouter"]
    Img --> P1
    Img --> P2
    Img --> P3
    Img --> P4
```

Used to draft transactions from free text or receipt images; still persisted through normal transaction services.

## MCP (Model Context Protocol)

```mermaid
sequenceDiagram
    participant Agent as MCP client
    participant Gin as /mcp
    participant Auth as JWTMCPAuthorization
    participant H as MCP tool handlers
    participant S as services

    Agent->>Gin: JSON-RPC initialize / tools.list / tools.call
    Gin->>Auth: MCP token + IP limit
    Auth->>H: dispatch method
    H->>S: add/query transactions, accounts, categories, tags, FX
    S-->>H: result
    H-->>Agent: JSON-RPC response
```

Registered tools (representative):

- `add_transaction`
- `query_transactions`
- `query_all_accounts` / `query_all_accounts_balance`
- `query_all_transaction_categories`
- `query_all_transaction_tags`
- `query_latest_exchange_rates`

Schemas generated via `invopop/jsonschema` on Go types.

## Object storage

```mermaid
flowchart TB
    SC["StorageContainer"] --> A["Avatar storage"]
    SC --> P["Transaction picture storage"]

    A --> Local["Local filesystem"]
    A --> MinIO["MinIO / S3 API"]
    A --> WebDAV["WebDAV"]
    P --> Local
    P --> MinIO
    P --> WebDAV
```

Interface: `Exists` / `Read` / `Save` / `Delete` (`pkg/storage`).

## Exchange rates

```mermaid
flowchart TB
    API["exchange_rates API"] --> Prov["ExchangeRatesDataProvider"]
    Prov --> HTTP["CommonHttpExchangeRatesDataProvider"]
    Prov --> Custom["UserCustomExchangeRatesDataProvider"]
    HTTP --> Banks["ECB, BoC, Norges Bank,<br/>SNB, many national banks…"]
    Custom --> DB["UserCustomExchangeRate rows"]
```

## Import / export formats

```mermaid
flowchart LR
    subgraph Import
        CSV["CSV / Excel / custom"]
        Bank["OFX QFX QIF IIF"]
        ISO["Camt.052/053 MT940"]
        Apps["GnuCash Firefly Beancount"]
        Region["Alipay WeChat Feidee JD …"]
        AI["AI-assisted parse"]
    end

    Import --> Pipeline["converters → datatable → importer"]
    Pipeline --> Tx["Transactions"]

    Tx --> Export["CSV / TSV export"]
```

## Mail & maps

```mermaid
flowchart LR
    Mail["pkg/mail SMTP"] --> Use["Verify email · reset password · 2FA mail"]
    Maps["Map providers"] --> Proxy["Optional tile proxy<br/>/proxy/map …"]
    Maps --> Amap["Amap proxy + cookie auth"]
```

## Cross-cutting: duplicate checker

Used for:

- Login / sensitive action rate limiting
- OAuth state / form idempotency
- Cron singleton coordination across instances

## Integration decision style

```mermaid
flowchart TB
    Cfg["settings.Config field"] --> Switch["Initialize* selects provider"]
    Switch --> IFace["Interface in package"]
    IFace --> Impl["Concrete provider package"]
    Impl --> HTTP["httpclient / SDK"]
```

Adding a new LLM, OAuth, FX, or storage backend means implementing the interface and wiring a config case — not changing API contracts.
