# Wishlist (Portfolio Code) + AWS Cost Guardrails

## Overview

Two independent deliverables shipped together:

**A. Wishlist feature.** Let a user track companies they do not own yet, without accounts, email, or passwords. Identity is a single generated secret — a "portfolio code" — that the browser stores locally and sends with each request. The server keeps only `sha256(code)`, so a database leak yields no usable keys. Wishlist entries are symbol + optional note + optional target price.

**B. Cost guardrails.** The current stack has no spending ceiling. API Gateway is throttled at 10 rps, which bounds Lambda, but the CloudFront default behavior serves the SPA straight from edge cache and is completely unbounded. CloudFront has no spend cap setting; the only real ceiling is an alarm that disables the distribution. This is pre-existing exposure, unrelated to the wishlist, but is being fixed in the same pass.

Explicitly out of scope: positions/lots in DynamoDB. Imported broker transactions remain the single source of truth for holdings. Storing positions would create a second source of truth requiring reconciliation.

## Context (from discovery)

Files/components involved:
- `infra/stack.go` — CDK: Lambda (256 MB, 29 s) → API Gateway (10 rps / burst 20) → CloudFront (S3 SPA default behavior, `/api/*` passthrough with `CACHING_DISABLED`). No DynamoDB, no alarms, no WAF, no reserved concurrency.
- `infra/main.go` — single stack, hardcoded region `eu-central-1`.
- `cmd/server/main.go` — one `net/http` mux; handlers `handleUpload`, `handleTicker`, `handleSearch`, `handleHistory`.
- `web/src/App.tsx` — `TABS` array drives `TabId` union and both desktop/mobile nav.
- `web/src/services/portfolioService.ts` — all API access.

Patterns found:
- Handlers are plain `func(w http.ResponseWriter, r *http.Request)`, method-checked with `http.Error(w, "method not allowed", http.StatusMethodNotAllowed)`, responses via `json.NewEncoder(w).Encode(...)` with the error deliberately discarded (`_ =`).
- Business logic lives in `internal/<pkg>`; `cmd/server` is thin wiring.
- Tests are table-driven, live beside the code in `internal/`, run via `make test`.

Dependencies identified:
- New: `github.com/aws/aws-sdk-go-v2/service/dynamodb` + `feature/dynamodb/attributevalue` (main module).
- New: `awscdk/awsdynamodb`, `awscdk/awscloudwatch`, `awscdk/awscloudwatchactions`, `awscdk/awssns`, `awscdk/awssnssubscriptions`, `awscdk/awsbudgets` (infra module).

Gaps that shape the plan:
- `cmd/server` has **no** test files today. New store logic goes in `internal/wishlist` so it is testable in the established style; handler tests use `net/http/httptest`.
- No frontend test runner exists. No frontend tests will be added; this is called out rather than silently skipped.
- **CloudFront CloudWatch metrics are published only in `us-east-1`.** The main stack is `eu-central-1`. The alarm therefore requires a second stack in `us-east-1`.

## Development Approach

- **testing approach**: Regular (implementation first, then tests) — chosen by user.
- complete each task fully before moving to the next
- make small, focused changes
- **every task with Go code changes MUST include new/updated tests** covering success and error paths
- **all tests must pass before starting the next task**
- CDK-only tasks are verified with `make cdk-diff` (no unit tests — CDK snapshot testing is not set up in this repo and adding it is out of scope)
- **update this plan file when scope changes during implementation**
- maintain backward compatibility: every existing endpoint and tab keeps working unchanged

## Testing Strategy

- **unit tests**: required for `internal/wishlist` (code generation, validation, store logic against a fake DynamoDB client interface).
- **handler tests**: `net/http/httptest` against the mux, using the same fake client. Covers auth-failure, validation-failure, and cap-enforcement paths — not just happy path.
- **e2e tests**: none. Project has no Playwright/Cypress/Vitest setup. Frontend changes are verified manually (see Post-Completion). Not adding a test runner in this plan.
- **infra**: `make cdk-diff` must show only the intended additions and must synth clean.

## Progress Tracking

- mark completed items with `[x]` immediately when done
- add newly discovered tasks with ➕ prefix
- document issues/blockers with ⚠️ prefix
- keep this plan in sync with actual work

## Solution Overview

### Identity model

A portfolio code is a **capability**: possession equals access. No recovery, no reset — stated plainly in the UI at generation time.

- 80 bits from `crypto/rand` (10 bytes), encoded Crockford base32, displayed grouped: `K7M2-9QRF-3XVB-8TDW`.
- Crockford alphabet excludes `I`, `L`, `O`, `U` — no visual ambiguity, and no accidental words.
- Implemented as 80 bits, not the 128 originally written here: 128 bits encodes to 26 characters, contradicting the 16-character example above. 10 bytes encodes to exactly 16 characters with no padding, and 80 bits is unguessable against a 10 rps throttle by an enormous margin.
- Server stores `sha256(code)` only. Normalization (uppercase, strip `-`) happens before hashing so display formatting never affects lookup.

### Why the code travels in a header, not the URL

**This is the most important security decision in the plan.** The code goes in an `X-Portfolio-Code` request header, never in the path or query string. URLs leak into CloudFront access logs, API Gateway logs, browser history, and `Referer` headers on outbound links. A header does not.

Consequence: `/api/wishlist` is a fixed path for all users, and every request body/response is scoped by the header. This also keeps the CloudFront `/api/*` behavior at `CACHING_DISABLED` — caching a code-scoped response would risk serving one user's wishlist to another if the cache key were ever misconfigured.

### Storage

Single table, provisioned **1 RCU / 1 WCU** — a hard cost ceiling of roughly $0.47/mo (inside free tier). Over-traffic returns `ProvisionedThroughputExceededException` rather than generating a bill. Autoscaling must NOT be enabled; PITR and Streams stay off.

```
PK (S) = "P#<sha256hex(code)>"
SK (S) = "META"            → createdAt, lastSeen, ttl
         "W#<SYMBOL>"      → symbol, note, targetPrice, addedAt
```

A single `Query` on PK returns metadata plus every wishlist row in one read.

TTL attribute on `META` set to `lastSeen + 12 months`, refreshed on write. Abandoned portfolios self-delete at no cost.

Abuse caps, enforced server-side: **50 symbols per portfolio**, note **≤ 500 chars**, symbol **≤ 12 chars** matching `^[A-Z0-9.\-]+$`. Without these the table becomes a free key-value store for anyone who wants one.

### Cost guardrails

Three layers, cheapest and highest-value first:

1. **Lambda `ReservedConcurrentExecutions: 5`** — free. Hard ceiling on concurrent compute regardless of what API Gateway lets through.
2. **CloudFront kill switch** — CloudWatch alarm on `BytesDownloaded` → SNS → inline Node.js Lambda that sets `Enabled: false` on the distribution. Reacts in ~2–5 min, propagation ~10 min. Caps a worst-case burst at roughly $10–15 instead of open-ended. The site goes down until manually re-enabled; for a personal app that is the correct tradeoff.
3. **AWS Budget + SNS email** — backup only. Billing data lags 8–24 h, so it catches a slow bleed, not a burst.

WAF (a rate-based rule, ~$6–8/mo) is the only option that prevents rather than detects, but costs more than the app does. **Deliberately excluded** — revisit if the app becomes genuinely public.

### Cross-region constraint

CloudFront publishes metrics only to `us-east-1`. A second stack `BrokersSyncCostGuardStack` is created there, receiving the distribution ID from the main stack. Both stacks get `CrossRegionReferences: jsii.Bool(true)`. AWS Budgets is a global service and is co-located in the guard stack for cohesion.

## Technical Details

### Assumptions requiring confirmation before deploy

These are parameterized via CDK context rather than hardcoded:

- **Alert email** — `brokersync@dot-core.com`. Default value for context key `alertEmail`, overridable at deploy time. Synth fails loudly if the key is explicitly blanked, rather than deploying a silent alarm.
- **Budget threshold** — **$5/mo** (confirmed), override via context `budgetLimitUsd`.
- **Alarm threshold** — default **5 GB of `BytesDownloaded` in 5 minutes**, override via context `bytesAlarmGb`. Normal use is ~0.5 GB/month, so this is roughly 4 orders of magnitude above baseline; false positives are implausible.

### API surface

All under `/api/wishlist`. Auth on every call: `X-Portfolio-Code` header.

| Method | Body | Response |
|---|---|---|
| `POST /api/wishlist/new` | none (no header required) | `{"code": "K7M2-9QRF-3XVB-8TDW"}` — generates and persists `META` |
| `GET /api/wishlist` | none | `{"items": [...]}` |
| `PUT /api/wishlist` | `{"symbol","note","targetPrice"}` | `204` — upsert |
| `DELETE /api/wishlist?symbol=X` | none | `204` |

Error contract: a missing, malformed, or unknown code all return an identical **`404`** with no body distinction. Distinguishing them would confirm which codes exist.

Note that `symbol` on DELETE is in the query string — that is fine, symbols are not secret. The *code* is never in a URL.

### Go package layout

```
internal/wishlist/
  code.go        NewCode() (string, error); Normalize(string) (string, bool); HashKey(string) string
  code_test.go
  store.go       type API interface { Query/PutItem/DeleteItem/UpdateItem }  ← narrow interface, fake-able
                 type Store struct { db API; table string }
                 EnsureMeta / List / Upsert / Delete
  store_test.go  fake API implementation, no AWS calls
```

The narrow `API` interface (only the four operations used) is what makes tests possible without network or LocalStack.

### Frontend

- `web/src/services/wishlistService.ts` — code in `localStorage` under `bs.portfolioCode`, injected as a header by a small `wishlistFetch` wrapper. `localStorage` not a cookie: no ambient authority, so no CSRF surface.
- `web/src/tabs/WishlistTab.tsx` — registered in the `TABS` array in `App.tsx`, which drives both nav renderers automatically.
- Empty state offers "Create a portfolio code" or "I already have one". After creation, the code is shown once with a copy button and an unambiguous warning that it cannot be recovered.
- Symbol entry reuses the existing `/api/search` endpoint; clicking a row reuses the existing ticker modal.

## What Goes Where

- **Implementation Steps** — code, tests, and CDK changes in this repo.
- **Post-Completion** — deploy, manual browser verification, and the one-time SNS email subscription confirmation.

## Implementation Steps

### Task 1: Portfolio code generation and normalization

**Files:**
- Create: `internal/wishlist/code.go`
- Create: `internal/wishlist/code_test.go`

- [ ] implement `NewCode()` — 16 bytes from `crypto/rand`, Crockford base32 encode, group into 4 blocks of 4 with `-`
- [ ] implement `Normalize(s string) (string, bool)` — uppercase, strip `-` and spaces, reject anything not 26 Crockford chars; map visually-confusable input (`I`/`L`→`1`, `O`→`0`) before validating
- [ ] implement `HashKey(code string) string` — `sha256` hex of the normalized form, prefixed `P#`
- [ ] write tests: `NewCode` output round-trips through `Normalize`, and 1000 generated codes are all distinct
- [ ] write tests: `Normalize` accepts lowercase/ungrouped/confusable input and rejects wrong length, empty, and non-alphabet chars
- [ ] write test asserting `HashKey` is stable across formatting variants of the same code (this is what makes display formatting safe to change later)
- [ ] run `make test` — must pass before Task 2

### Task 2: DynamoDB store layer

**Files:**
- Create: `internal/wishlist/store.go`
- Create: `internal/wishlist/store_test.go`
- Modify: `go.mod`, `go.sum`

- [ ] add `aws-sdk-go-v2/service/dynamodb` and `feature/dynamodb/attributevalue` deps
- [ ] define narrow `API` interface covering only `Query`, `PutItem`, `UpdateItem`, `DeleteItem`
- [ ] implement `EnsureMeta(ctx, code)` — writes `META` row with `createdAt`, `lastSeen`, `ttl`; `attribute_not_exists` condition so it never clobbers an existing portfolio
- [ ] implement `List(ctx, code)` — single `Query` on PK; returns `ErrNotFound` when no `META` row exists
- [ ] implement `Upsert(ctx, code, item)` — validates symbol pattern and note length, enforces the 50-symbol cap, refreshes `lastSeen`/`ttl`
- [ ] implement `Delete(ctx, code, symbol)`
- [ ] write tests with a fake `API`: `EnsureMeta` is idempotent and does not overwrite `createdAt`
- [ ] write tests: `List` on an unknown code returns `ErrNotFound` (encodes *why* — unknown and missing codes must be indistinguishable to callers)
- [ ] write tests: `Upsert` rejects a 51st symbol, an over-long note, and a malformed symbol (each cap exists to stop the table being used as free storage)
- [ ] write test: `Upsert` refreshes `ttl` (encodes that active portfolios must not expire)
- [ ] run `make test` — must pass before Task 3

### Task 3: HTTP handlers

**Files:**
- Modify: `cmd/server/main.go`
- Create: `cmd/server/wishlist_handlers.go`
- Create: `cmd/server/wishlist_handlers_test.go`

- [ ] add `handleWishlistNew` (`POST /api/wishlist/new`) — generates code, calls `EnsureMeta`, returns `{"code": ...}`
- [ ] add `handleWishlist` (`GET`/`PUT`/`DELETE` on `/api/wishlist`) dispatching on method, matching the existing `http.Error(..., "method not allowed", ...)` convention
- [ ] extract the `X-Portfolio-Code` header, `Normalize` it, and return a bare `404` on missing/malformed/unknown — one shared code path so the three cases cannot diverge
- [ ] cap request body at 8 KB via `http.MaxBytesReader`
- [ ] register routes in the mux in `cmd/server/main.go`; construct the store from `WISHLIST_TABLE` env var, and register the routes only when it is set (so local runs without AWS still work)
- [ ] write `httptest` tests: full create → list → upsert → delete cycle returns expected status codes
- [ ] write `httptest` tests: no header, malformed header, and unknown-but-well-formed code all return `404` with **byte-identical** responses (this is the anti-enumeration guarantee — a test that fails the moment someone adds a helpful error message)
- [ ] write `httptest` test: oversized body is rejected
- [ ] run `make test` — must pass before Task 4

### Task 4: DynamoDB table and Lambda wiring in CDK

**Files:**
- Modify: `infra/stack.go`
- Modify: `infra/go.mod`, `infra/go.sum`

- [ ] add `awsdynamodb.NewTable` — PK `PK` (S), SK `SK` (S), `BillingMode_PROVISIONED`, `ReadCapacity: 1`, `WriteCapacity: 1`, `TimeToLiveAttribute: "ttl"`, `RemovalPolicy_RETAIN`
- [ ] confirm no autoscaling call is added and PITR is left off — the fixed capacity is the entire cost guarantee
- [ ] `table.GrantReadWriteData(lambdaFn)`
- [ ] pass `WISHLIST_TABLE: table.TableName()` in the Lambda environment
- [ ] set `ReservedConcurrentExecutions: jsii.Number(5)` on the Lambda (guardrail 1)
- [ ] run `make cdk-diff` — verify it shows exactly one new table, the IAM grant, the env var, and the concurrency setting, with no unrelated drift

### Task 5: CloudFront kill-switch stack (us-east-1)

**Files:**
- Create: `infra/costguard_stack.go`
- Modify: `infra/main.go`
- Modify: `infra/stack.go`

- [ ] add `CrossRegionReferences: jsii.Bool(true)` to both stacks and export the distribution ID from the main stack
- [ ] create `NewCostGuardStack` in `us-east-1` (required — CloudFront metrics exist nowhere else)
- [ ] read context keys with defaults: `alertEmail` (default `brokersync@dot-core.com`), `budgetLimitUsd` (default 5), `bytesAlarmGb` (default 5); **panic at synth time if `alertEmail` resolves to empty** — a silently unsubscribed alarm is worse than no alarm
- [ ] create SNS topic with an email subscription to `alertEmail`
- [ ] create the kill-switch Lambda: `Runtime_NODEJS_22_X`, `Code_FromInline`, reads the distribution ID from env, calls `GetDistributionConfig` then `UpdateDistribution` with `Enabled: false` (inline Node avoids adding a bundler to a Go CDK app)
- [ ] grant it `cloudfront:GetDistributionConfig` and `cloudfront:UpdateDistribution`, scoped to the single distribution ARN
- [ ] create the `BytesDownloaded` alarm (`AWS/CloudFront`, `DistributionId` + `Region: Global`, 5-min period, threshold `bytesAlarmGb`), with both the SNS topic and the Lambda as alarm actions
- [ ] add `awsbudgets.CfnBudget` — monthly cost budget at `budgetLimitUsd`, notifications at 80% actual and 100% forecasted, to the same SNS topic (guardrail 2)
- [ ] run `make cdk-diff` — both stacks synth clean; verify the guard stack resolves to `us-east-1`

### Task 6: Wishlist frontend service and tab

**Files:**
- Create: `web/src/services/wishlistService.ts`
- Create: `web/src/tabs/WishlistTab.tsx`
- Modify: `web/src/App.tsx`

- [ ] implement `wishlistService.ts` — `localStorage` key `bs.portfolioCode`, a `wishlistFetch` wrapper injecting `X-Portfolio-Code`, and `createCode` / `list` / `upsert` / `remove`
- [ ] treat a `404` from any call as "code invalid" — clear local state and return to the empty state rather than showing a raw error
- [ ] build `WishlistTab.tsx`: empty state ("Create a code" / "I have a code"), one-time code reveal with copy button and an explicit unrecoverable warning, and the symbol table
- [ ] wire symbol lookup to the existing `/api/search` endpoint and row clicks to the existing ticker modal
- [ ] add note and target-price editing, enforcing the same 500-char limit client-side (server remains authoritative)
- [ ] register the tab in the `TABS` array in `App.tsx` — desktop and mobile nav pick it up automatically
- [ ] run `make build-web` — must compile clean before Task 7

⚠️ No frontend tests: this repo has no JS test runner, and adding one is outside this plan's scope. Frontend verification is manual (see Post-Completion). Flagging rather than quietly skipping.

### Task 7: Verify acceptance criteria

- [ ] verify a fresh code creates, persists, and lists correctly
- [ ] verify a wrong code is indistinguishable from an unknown one at the HTTP level
- [ ] verify the 50-symbol and 500-char caps reject at the boundary
- [ ] verify existing tabs (Overview, Brokers, Positions, Trades, PnL, Dividends) are untouched
- [ ] grep the code to confirm the portfolio code appears in no URL path, query string, or log statement
- [ ] run full test suite: `make test`
- [ ] run `make lint` and `make fmt`
- [ ] run `make cdk-diff` and re-read the full diff for unintended changes
- [ ] no e2e suite to run (none exists in this project)

### Task 8: [Final] Update documentation

- [ ] document the wishlist endpoints and the `X-Portfolio-Code` header in `README.md`
- [ ] document the required CDK context keys (`alertEmail`, `budgetLimitUsd`, `bytesAlarmGb`) and the deploy command in `README.md`
- [ ] add a short note to `README.md` on what happens when the kill switch fires and how to re-enable the distribution
- [ ] update `CLAUDE.md` only if new conventions emerged
- [ ] move this plan to `docs/plans/completed/`

## Post-Completion

*Manual/external — no checkboxes*

**Deploy:**
- `make cdk-deploy` works with no extra context — `alertEmail` defaults to `brokersync@dot-core.com`. Override with `--context alertEmail=other@example.com` if needed.
- The SNS email subscription requires a **one-time click** in a confirmation email sent to `brokersync@dot-core.com`. Until confirmed, the alarm fires but no email arrives. Verify explicitly.

**Manual verification:**
- Full browser flow: create code → add symbols → reload page (code persists) → open in a second browser and paste the code (same list appears) → clear `localStorage` (returns to empty state).
- Confirm the code appears in no URL by watching the browser network tab during the whole flow.

**Kill-switch test (recommended, and easy to forget):**
- Temporarily set the alarm threshold very low, trigger it, and confirm the distribution actually disables and the email arrives. An untested kill switch is an assumption, not a guardrail. Restore the threshold and re-enable the distribution afterwards.

**Ongoing:**
- After a month, check the actual DynamoDB and CloudFront line items against the ~$0.50/mo expectation.
- If the app ever becomes genuinely public, revisit the WAF rate-based rule that was deliberately excluded here.
