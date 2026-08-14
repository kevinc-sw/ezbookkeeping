# System overview

## Runtime topology

```mermaid
flowchart TB
    subgraph Deploy["Deployment unit"]
        Bin["ezbookkeeping binary"]
        Static["Embedded / static root<br/>desktop + mobile assets, sw.js"]
        Conf["conf/*.ini"]
    end

    subgraph Process["Process"]
        HTTP["HTTP listener :8080"]
        Scheduler["gocron scheduler"]
        Containers["Singleton containers<br/>settings, datastore, storage,<br/>llm, mail, uuid, oauth2, mcp, cron…"]
    end

    Bin --> HTTP
    Bin --> Scheduler
    Conf --> Containers
    Static --> HTTP
    Containers --> HTTP
    Containers --> Scheduler
```

## CLI surface

Entry: `ezbookkeeping.go` → `urfave/cli/v3`.

```mermaid
flowchart LR
    Main["ezbookkeeping"] --> Server["server run"]
    Main --> Database["database …"]
    Main --> UserData["user_data …"]
    Main --> CronJobs["cron_jobs …"]
    Main --> Security["security …"]
    Main --> Utils["utilities …"]
```

| Command | Role |
| --- | --- |
| `server run` | Boot containers, optionally sync schema, serve HTTP + cron |
| `database` | Schema sync / DB maintenance |
| `user_data` | Import/export/clear user data from CLI |
| `cron_jobs` | Run scheduled jobs outside the web process |
| `security` / `utilities` | Ops helpers |

## Boot sequence (`server run`)

```mermaid
sequenceDiagram
    participant CLI as cmd/webserver
    participant Init as cmd/initializer
    participant Gin as Gin router
    participant Cron as pkg/cron

    CLI->>Init: initializeSystem
    Init->>Init: settings → datastore → log
    Init->>Init: storage → llm → uuid
    Init->>Init: duplicatechecker → avatars → mail → FX
    CLI->>CLI: optional AutoUpdateDatabase (SyncStructs)
    CLI->>CLI: requestid, mcp, oauth2
    CLI->>Cron: InitializeCronJobSchedulerContainer(start=true)
    CLI->>Gin: register routes + middleware
    CLI->>Gin: ListenAndServe
```

Key files:

- `ezbookkeeping.go`
- `cmd/initializer.go`
- `cmd/webserver.go`
- `cmd/database.go`

## HTTP surface (logical)

```mermaid
flowchart TB
    Gin["Gin"]

    Gin --> SPA["SPA / static<br/>/, /mobile, /desktop, assets, sw.js"]
    Gin --> SettingsJS["/server_settings.js"]
    Gin --> Health["/healthz.json"]
    Gin --> Assets["/avatar, /pictures, /proxy/map, QR"]
    Gin --> OAuthRoutes["/oauth2/login, /oauth2/callback"]
    Gin --> MCP["/mcp JSON-RPC"]
    Gin --> API["/api …"]

    API --> Public["Public auth<br/>authorize, register, forget password…"]
    API --> Special["Token-typed routes<br/>2FA, OAuth callback, email verify, reset"]
    API --> V1["/api/v1/* JWT + IP limits"]
```

## Persistence split

```mermaid
flowchart LR
    Svc["services"] --> DS["DataStoreContainer"]
    DS --> UserStore["UserStore"]
    DS --> TokenStore["TokenStore"]
    DS --> UserDataStore["UserDataStore"]

    UserStore --> DB[(Same DB today)]
    TokenStore --> DB
    UserDataStore --> DB

    Svc --> SC["StorageContainer"]
    SC --> Avatar["Avatar ObjectStorage"]
    SC --> Pics["Transaction picture ObjectStorage"]
```

`DataStore.Choose(uid)` is sharding-ready but currently always returns the first engine.

## Deployment shapes

```mermaid
flowchart TB
    subgraph Docker["Docker (typical)"]
        D1["mayswind/ezbookkeeping"]
        D1 --> VolDB["Volume: DB file or external DB"]
        D1 --> VolData["Volume: local object storage path"]
        D1 --> VolConf["Config / env"]
    end

    subgraph Binary["Bare binary"]
        B1["./ezbookkeeping server run"]
        B1 --> Files["conf + static root + data dirs"]
    end
```

Supports SQLite (simple), MySQL, PostgreSQL; x86 / amd64 / ARM; low-resource hosts (Pi, NAS).
