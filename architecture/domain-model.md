# Domain model

User-scoped bookkeeping data. Soft delete via `Deleted` + `DeletedUnixTime`. Money as **int64 minor units**. Hierarchy on accounts and categories via parent IDs.

## Entity relationship

```mermaid
erDiagram
    User ||--o{ Account : owns
    User ||--o{ Transaction : owns
    User ||--o{ TransactionCategory : owns
    User ||--o{ TransactionTag : owns
    User ||--o{ TransactionTagGroup : owns
    User ||--o{ TransactionTemplate : owns
    User ||--o{ TransactionPictureInfo : owns
    User ||--o{ TokenRecord : has
    User ||--o| TwoFactor : has
    User ||--o{ TwoFactorRecoveryCode : has
    User ||--o{ UserExternalAuth : links
    User ||--o{ UserCustomExchangeRate : defines
    User ||--o{ UserApplicationCloudSetting : syncs
    User ||--o{ InsightsExplorer : saves

    Account ||--o{ Account : "parent / sub"
    Account ||--o{ Transaction : "AccountId"
    TransactionCategory ||--o{ TransactionCategory : "parent / child"
    TransactionCategory ||--o{ Transaction : "CategoryId"
    Transaction ||--o| Transaction : "transfer RelatedId"
    Transaction ||--o{ TransactionTagIndex : tagged
    TransactionTag ||--o{ TransactionTagIndex : tagged
    TransactionTagGroup ||--o{ TransactionTag : groups
    Transaction ||--o{ TransactionPictureInfo : attachments
    TransactionTemplate ||--o| Transaction : "creates scheduled"
```

## Core aggregates

### User & security

| Entity | Notes |
| --- | --- |
| `User` | Profile, defaults (currency, account, week/fiscal), feature restrictions |
| `TokenRecord` | Per-issued JWT secret → revoke by deleting row |
| `TwoFactor` / recovery codes | TOTP + backup codes |
| `UserExternalAuth` | OIDC / GitHub / Gitea / Nextcloud link |

### Accounts

- Categories: cash, checking, credit card, virtual, debt, receivables, investment, savings, CD, …
- Type: single vs multi-sub-accounts (`ParentAccountId`)
- `Balance` maintained with transactions; extend JSON for reconcile / statement date

### Transactions

```mermaid
flowchart LR
    subgraph Types
        MB["ModifyBalance"]
        IN["Income"]
        EX["Expense"]
        TO["TransferOut"]
        TI["TransferIn"]
    end

    TO -.RelatedId.-> TI
```

- Transfer = **two DB rows** linked by `RelatedId` / `RelatedAccountId` / amounts.
- Optional geo, timezone offset, comment, `ScheduledCreated`.
- Tags via `TransactionTagIndex` (denormalized `TransactionTime` for filters).

### Categories, tags, templates

- Categories: Income / Expense / Transfer; two-level tree.
- Tags optional groups; many-to-many with transactions.
- Templates: `Normal` or `Schedule` (frequency fields); cron materializes scheduled txs.

### Media & analytics

- `TransactionPictureInfo` metadata; blobs in object storage `uid/pictureId.ext`.
- `InsightsExplorer` — saved custom analytic dimensions / queries.

## Ownership & multi-tenancy

```mermaid
flowchart TB
    Uid["Uid on every user-data row"] --> Q["All queries scoped by Uid"]
    Q --> Iso["Logical tenant = user"]
    Iso --> Shard["Future: DataStore.Choose(Uid)"]
```

No shared household entities today — one user, one book.

## Amount & time conventions

```mermaid
flowchart LR
    UI["UI decimal"] --> Svc["Service converts"]
    Svc --> Int["int64 minor units in DB"]
    Int --> FX["FX convert using rate providers"]
    Ts["Unix times + timezone offset on tx"] --> Display["Client formats via i18n helpers"]
```

## Key model files

- `pkg/models/user.go`
- `pkg/models/account.go`
- `pkg/models/transaction.go`
- `pkg/models/transaction_category.go`
- `pkg/models/transaction_tag*.go`
- `pkg/models/transaction_template.go`
- `pkg/models/transaction_picture_info.go`
- `pkg/models/token_record.go`
- `pkg/models/explorer.go`

Frontend mirrors many of these under `src/models/` and pure logic under `src/core/`.
