# Backend architecture

Layering: **Gin adapters → `pkg/api` → `pkg/services` → `pkg/datastore` / `pkg/storage` / providers**.

## Package map

```mermaid
flowchart TB
    subgraph Entry
        CMD["cmd/"]
        MAIN["ezbookkeeping.go"]
    end

    subgraph HTTP
        MW["pkg/middlewares"]
        API["pkg/api"]
        MCP["pkg/mcp"]
    end

    subgraph Domain
        SVC["pkg/services"]
        MODELS["pkg/models"]
        CONV["pkg/converters"]
        CRON["pkg/cron"]
    end

    subgraph Infra
        DS["pkg/datastore"]
        ST["pkg/storage"]
        AUTH["pkg/auth/oauth2"]
        LLM["pkg/llm"]
        FX["pkg/exchangerates"]
        MAIL["pkg/mail"]
        CFG["pkg/settings"]
        CORE["pkg/core"]
        ERR["pkg/errs"]
    end

    MAIN --> CMD
    CMD --> MW
    CMD --> API
    CMD --> MCP
    CMD --> CRON
    MW --> SVC
    API --> SVC
    MCP --> SVC
    CRON --> SVC
    SVC --> MODELS
    SVC --> DS
    SVC --> ST
    SVC --> AUTH
    SVC --> LLM
    SVC --> FX
    SVC --> MAIL
    SVC --> CONV
    CMD --> CFG
    API --> CORE
    SVC --> CORE
    API --> ERR
```

## Request lifecycle (authenticated API)

```mermaid
sequenceDiagram
    participant C as Client
    participant G as Gin
    participant MW as Middlewares
    participant A as pkg/api
    participant S as pkg/services
    participant DB as XORM / DataStore

    C->>G: HTTP /api/v1/...
    G->>MW: Recovery
    G->>MW: RequestId
    G->>MW: RequestLog
    G->>MW: JWTAuthorizationByHeader
    MW->>S: Tokens.ParseToken (DB secret)
    G->>MW: APITokenIpLimit
    G->>A: bindApi → ApiHandlerFunc(WebContext)
    A->>A: Bind DTO / validate
    A->>S: business call
    S->>DB: Session / DoTransaction
    DB-->>S: rows
    S-->>A: domain result
    A-->>G: response DTO or errs.Error
    G-->>C: JSON success / error
```

Handler adapters in `cmd/webserver.go` keep Gin out of domain code:

| Binder | Use |
| --- | --- |
| `bindApi` | JSON success/error |
| `bindApiWithTokenUpdate` | Auth responses that rotate tokens |
| `bindJSONRPCApi` | MCP |
| `bindEventStreamApi` | Streaming |
| `bindCsv` / `bindTsv` | Exports |
| `bindImage` / `bindCachedImage` / `bindProxy` | Media & map proxy |
| `bindRedirect` / `bindLocalFile` | OAuth / static |

## Middleware chain (v1)

```mermaid
flowchart LR
    R[Recovery] --> RID[RequestId]
    RID --> RL[RequestLog]
    RL --> JWT[JWTAuthorizationByHeader]
    JWT --> IP[APITokenIpLimit]
    IP --> H[API handler]
```

Other JWT variants: `JWTTwoFactorAuthorization`, `JWTOAuth2CallbackAuthorization`, `JWTEmailVerifyAuthorization`, `JWTResetPasswordAuthorization`, `JWTMCPAuthorization`.

## Service composition

Services are singletons that **embed** capability structs (`pkg/services/base.go`):

```mermaid
classDiagram
    class ServiceUsingDB {
        +UserDataDB(uid)
        +DoTransaction(...)
    }
    class ServiceUsingConfig
    class ServiceUsingMailer
    class ServiceUsingUuid
    class ServiceUsingStorage

    class TransactionService
    class TokenService
    class UserService

    TransactionService --|> ServiceUsingDB
    TransactionService --|> ServiceUsingConfig
    TransactionService --|> ServiceUsingUuid
    TransactionService --|> ServiceUsingStorage
    TokenService --|> ServiceUsingDB
    TokenService --|> ServiceUsingConfig
    UserService --|> ServiceUsingDB
    UserService --|> ServiceUsingMailer
```

Same idea on the API side: `ApiUsingConfig`, `ApiUsingDuplicateChecker`, `ApiUsingAvatarProvider`, `ApiWithUserInfo`.

## Context abstraction

```mermaid
flowchart TB
    Ctx["core.Context"]
    Ctx --> Web["WebContext<br/>wraps gin.Context"]
    Ctx --> Cli["CliContext"]
    Ctx --> CronCtx["CronContext"]
    Ctx --> Null["NullContext"]

    Web --> Svc["services / datastore"]
    Cli --> Svc
    CronCtx --> Svc
```

One service implementation serves HTTP, CLI, and cron.

## Auth model (JWT + DB)

```mermaid
flowchart TB
    Login["Login / OAuth / 2FA"] --> Issue["TokenService issues JWT"]
    Issue --> Rec["TokenRecord row<br/>per-token secret + type + expiry"]
    Issue --> JWT["JWT (claims + token id)"]

    Req["API request"] --> Parse["ParseToken"]
    Parse --> Lookup["Load TokenRecord by claims"]
    Lookup --> Verify["HMAC with DB secret"]
    Verify --> Claims["UserTokenClaims on WebContext"]

    Revoke["Revoke / logout"] --> Del["Delete TokenRecord"]
```

Token types include NORMAL, API, MCP, REQUIRE_2FA, OAUTH2_CALLBACK, EMAIL_VERIFY, PASSWORD_RESET (see `pkg/core`).

## Cron jobs

Scheduler: `gocron` v2 via `pkg/cron`. Notable job: **scheduled transactions** (~every 15 min) → `services.Transactions.CreateScheduledTransactions`. Duplicate-checker can act as a cross-instance lock.

## Converters pipeline

```mermaid
flowchart LR
    File["Uploaded file"] --> Fmt["Format-specific parser<br/>csv/qif/ofx/camt/…"]
    Fmt --> DT["datatable layer<br/>normalized rows"]
    DT --> Imp["DataTableTransactionDataImporter"]
    Imp --> Resolve["Resolve accounts / categories / tags"]
    Imp --> Out["ImportedTransactionSlice"]
    Out --> Svc["Transaction service persist"]
```

Dispatcher: `pkg/converters/transaction_data_converters.go`.

## Error & response contract

- Domain errors: `pkg/errs.Error` with numeric codes.
- Success/error JSON helpers: `pkg/utils` (`PrintJsonSuccessResult` / `PrintJsonErrorResult`).
- Frontend maps known codes in `src/consts/api.ts`.
