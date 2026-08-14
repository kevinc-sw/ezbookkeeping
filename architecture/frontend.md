# Frontend architecture

Vue 3 + TypeScript. One `src/` tree builds **three** HTML entries; desktop and mobile share stores, models, and `*Base` logic.

## Entry points

```mermaid
flowchart TB
    Index["index.html<br/>index-main.ts"] -->|"UA sniff<br/>ua-parser-js"| Decision{mobile / wearable / embedded?}
    Decision -->|yes| Mobile["mobile.html<br/>mobile-main.ts"]
    Decision -->|no| Desktop["desktop.html<br/>desktop-main.ts"]

    Desktop --> DStack["Vue Router + Vuetify<br/>ECharts + Pinia + i18n"]
    Mobile --> MStack["Framework7 + framework7-vue<br/>Pinia + i18n"]

    DStack --> Shared["Shared core<br/>stores · models · lib · locales"]
    MStack --> Shared
    Shared --> API["axios → /api"]
```

| Entry | Shell | UI kit |
| --- | --- | --- |
| `src/desktop-main.ts` | `DesktopApp.vue` | Vuetify |
| `src/mobile-main.ts` | `MobileApp.vue` | Framework7 (iOS theme) |
| `src/index-main.ts` | redirect only | — |

## Logical layers

```mermaid
flowchart TB
    subgraph UI
        Views["views/desktop · views/mobile"]
        Comp["components/desktop · mobile · common"]
        Bases["views/base · components/base"]
    end

    subgraph State
        Pinia["Pinia stores"]
        UserState["lib/userstate.ts<br/>token + app lock"]
    end

    subgraph AppServices
        Lib["lib/* helpers"]
        Services["lib/services.ts axios facade"]
        Core["core/* pure domain"]
        Models["models/* DTOs + classes"]
        Consts["consts/* catalogs"]
    end

    subgraph Browser
        SW["sw.ts Workbox PWA"]
        LS["localStorage / sessionStorage"]
    end

    Views --> Bases
    Comp --> Bases
    Views --> Pinia
    Comp --> Pinia
    Pinia --> Services
    Pinia --> Lib
    Pinia --> UserState
    Services --> Models
    Lib --> Core
    UserState --> LS
    Views --> SW
```

## Shared vs platform-specific

```mermaid
flowchart LR
    subgraph Shared
        VB["views/base/*PageBase.ts"]
        CB["components/base/*Base.ts"]
        CC["components/common/*"]
        Stores["stores/*"]
        Models["models/*"]
    end

    subgraph Desktop
        VD["views/desktop/*"]
        CD["components/desktop/*"]
    end

    subgraph Mobile
        VM["views/mobile/*"]
        CM["components/mobile/*"]
    end

    VB --> VD
    VB --> VM
    CB --> CD
    CB --> CM
    CC --> VD
    CC --> VM
    Stores --> VD
    Stores --> VM
```

Pattern: business logic in `*Base.ts`; thin SFC templates per platform.

## Pinia store map

```mermaid
flowchart TB
    Root["root<br/>stores/index.ts<br/>auth orchestration · notifications · resetAllStates"]

    Root --> User["user"]
    Root --> Token["token"]
    Root --> Settings["settings"]
    Root --> TwoFA["twoFactorAuth"]
    Root --> ExtAuth["userExternalAuth"]
    Root --> Accounts["accounts"]
    Root --> Tx["transactions"]
    Root --> Cats["transactionCategories"]
    Root --> Tags["transactionTags"]
    Root --> Tpl["transactionTemplates"]
    Root --> Overview["overview"]
    Root --> Stats["statistics"]
    Root --> Explore["explorers"]
    Root --> FX["exchangeRates"]
    Root --> Sys["systems"]
    Root --> Env["environments"]
    Root --> Desk["desktopPages"]
```

## API client (`lib/services.ts`)

```mermaid
sequenceDiagram
    participant Page as View / Store
    participant Ax as axios + interceptors
    participant US as userstate
    participant API as Backend /api

    Page->>Ax: method call
    Ax->>US: getCurrentToken()
    Ax->>API: Bearer + timezone headers
    alt token error 202001–202006 / 202012
        API-->>Ax: error
        Ax->>US: clearCurrentTokenAndUserInfo
        Ax->>Ax: location.reload()
    else success
        API-->>Ax: ApiResponse&lt;T&gt;
        Ax-->>Page: data
    end
```

Also supports: request blocking during token refresh, cancelable requests, per-operation timeouts (import/LLM/export longer than CRUD).

## Routing (guards)

```mermaid
flowchart TB
    subgraph Desktop["vue-router hash #/"]
        DL["MainLayout + children"]
        DA["Auth pages outside layout"]
        DL --> G1["checkLogin / checkLocked"]
        DA --> G2["checkNotLogin where needed"]
    end

    subgraph Mobile["Framework7 #!/"]
        MR["asyncResolve(component)"]
        MR --> G3["same login / lock guards"]
    end
```

App lock (PIN / WebAuthn) sits in `lib/userstate.ts` + unlock routes, independent of server JWT.

## PWA / service worker

```mermaid
flowchart TB
    SW["sw.ts injectManifest"]
    SW --> Precache["precacheAndRoute(__WB_MANIFEST)"]
    SW --> Assets["img/fonts → StaleWhileRevalidate / CacheFirst"]
    SW --> Code["html/js/css → NetworkFirst / CacheFirst"]
    SW --> Map["map tiles → CacheFirst + strip token"]
    SW --> Share["POST __share__image__ → cache blob + 303"]

    App["DesktopApp / MobileApp"] -->|register sw.js| SW
    CacheLib["lib/cache.ts"] -->|UPDATE_MAP_CACHE_CONFIG| SW
```

## i18n

- Bundles under `src/locales/` (~20 languages).
- `vue-i18n` + large `locales/helpers.ts` for currency/date/numeral formatting.
- Desktop bridges locale/RTL into Vuetify; mobile uses separate LTR/RTL SCSS entries.

## Notable frontend choices

1. **Dual SPA, not responsive single app** — different navigation metaphors (sidebar vs F7 sheets).
2. **Hash history** — works behind simple static hosting / reverse proxies without rewrite rules.
3. **Stores own orchestration**; views stay thin via `*PageBase`.
4. **PWA share target** enables “share receipt image to app” on mobile.
