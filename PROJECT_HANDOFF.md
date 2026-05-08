# Housecall Pro CLI Handoff

Handoff generated: 2026-05-08  
Project folder: `/Users/gvh41/Desktop/Rozeta Labs/Code Projects/hcp-cli`  
Linear project: https://linear.app/rozeta-labs/project/housecallpro-cli-v1-c9cf7cf595f5

## Current State

`hcp` is now a Go/Cobra CLI with an initial Housecall Pro auth layer, typed/shared API client, SQLite local mirror, bounded/incremental sync path, local-first list commands, a generic natural-language/direct API command, local operator reports, delivery dry-runs, and an MCP server shell.

The repo is not currently initialized as a git repository. `git status` returns `fatal: not a git repository`.

## Implemented

Core project/reference docs:

- `Housecall_Pro_CLI_V1_Brief_Rozeta_Labs.md`
- `PRINTING_PRESS_GUIDE.md`
- `HOUSECALL_PRO_API_REFERENCE.md`
- `HOUSECALL_PRO_OPENAPI_SNAPSHOT.yaml`
- `API_COVERAGE_GAP_ANALYSIS.md`
- `AGENT_API_GUIDE.md`
- `LINEAR_V1_PLAN.md`

Code:

- `cmd/hcp/main.go` — binary entrypoint.
- `internal/cli` — Cobra commands and output behavior.
- `internal/config` — local JSON config/auth storage.
- `internal/hcp` — Housecall Pro HTTP client.
- `internal/store` — SQLite mirror and typed upserts/reads.
- `internal/syncer` — bounded API-to-SQLite sync.
- `internal/cli/api.go` — generic `hcp api` command for natural-language and explicit Housecall Pro API calls.
- `internal/cli/reports.go` — local report commands for briefs, funnel, sales, cash, fulfillment, leakage detection, and delivery dry-runs.

Built binary:

- `bin/hcp`

## Commands That Work

Build and test:

```bash
go test ./...
go build -o bin/hcp ./cmd/hcp
```

Help:

```bash
./bin/hcp --help
./bin/hcp customers list --help
```

Auth/config:

```bash
./bin/hcp auth login --api-key <housecall-pro-api-key>
./bin/hcp auth status
./bin/hcp auth doctor --offline
./bin/hcp auth doctor
```

Sync:

```bash
./bin/hcp sync
./bin/hcp sync --resource customers --max-pages 1
./bin/hcp sync --resource customers --resource jobs --page-size 100 --max-pages 5
./bin/hcp sync --since yesterday --resource customers
./bin/hcp sync --watch --watch-interval 5m --watch-max-runs 3
./bin/hcp sync --dry-run --json
```

Local-first list commands:

```bash
./bin/hcp customers list
./bin/hcp jobs list
./bin/hcp estimates list
./bin/hcp leads list
./bin/hcp invoices list
```

Operator/report commands:

```bash
./bin/hcp brief today --json
./bin/hcp brief yesterday
./bin/hcp brief week
./bin/hcp brief month
./bin/hcp funnel --last 30d --json
./bin/hcp funnel leakage
./bin/hcp estimates open
./bin/hcp estimates unsold --last 30d
./bin/hcp estimates stale --days 3
./bin/hcp estimates high-value --min 5000
./bin/hcp estimates no-followup
./bin/hcp leads stale --minutes 15
./bin/hcp leads --uncontacted
./bin/hcp leads --unbooked
./bin/hcp jobs today
./bin/hcp jobs tomorrow
./bin/hcp jobs active
./bin/hcp jobs unscheduled
./bin/hcp jobs delayed
./bin/hcp jobs stalled
./bin/hcp jobs completed-not-invoiced
./bin/hcp invoices open
./bin/hcp invoices overdue
./bin/hcp invoices aging
./bin/hcp cash collected --yesterday
./bin/hcp report owner-daily --send slack --dry-run
./bin/hcp report owner-daily --send email --dry-run
./bin/hcp mcp serve --json
```

Direct live API reads are explicit:

```bash
./bin/hcp customers list --data-source live
```

Natural-language or explicit API actions:

```bash
./bin/hcp api list customers --limit 10 --plan
./bin/hcp api get /company
./bin/hcp api create customer --body '{"first_name":"Ada"}' --plan
./bin/hcp api create customer --body '{"first_name":"Ada"}' --yes
./bin/hcp api --method PATCH --path /customers/cus_123 --body '{"last_name":"Lovelace"}' --yes
./bin/hcp api create job appointment --param job_id=job_123 --body '{"start_time":"2026-05-09T09:00:00Z"}' --plan
./bin/hcp api add job attachment --param job_id=job_123 --file ./photo.jpg --file-field attachment --content-type image/jpeg --plan
./bin/hcp api catalog --json
./bin/hcp api catalog --area jobs
./bin/hcp api examples --json
./bin/hcp api examples --area estimates
```

`hcp api` can call any Housecall Pro endpoint by passing `--method` and `--path`. The natural-language planner recognizes common HCP resources and endpoint names from the OpenAPI snapshot. Mutating methods (`POST`, `PATCH`, `PUT`, `DELETE`) require `--yes` unless `--plan` or `--dry-run` is used. Destructive actions require an additional `--confirm <method:path>` token.

Local list commands support:

```bash
--data-source local
--data-source live
--db <path>
--json
--limit <n>
```

## Config And Storage

Config:

- Default config path comes from Go's `os.UserConfigDir`, under `hcp/config.json`.
- Override with `--config` or `HCP_CONFIG`.
- API key can be stored with `hcp auth login --api-key ...`.
- API key can also be provided with `HOUSECALL_PRO_API_KEY`.

Auth:

- API key mode sends `Authorization: Token <key>`.
- OAuth mode is structurally supported as `Authorization: Bearer <token>`, but V1 should keep API key mode as the primary path until partner OAuth is truly needed.

SQLite:

- Default DB path comes from Go's `os.UserCacheDir`, under `hcp/hcp.sqlite`.
- Override with `--db` or `HCP_DB`.
- The store uses `modernc.org/sqlite`, `PRAGMA busy_timeout = 5000`, WAL mode, and one connection per process.

Current tables:

- `sync_runs`
- `companies`
- `checklists`
- `customers`
- `customer_addresses`
- `leads`
- `lead_line_items`
- `jobs`
- `appointments`
- `job_line_items`
- `job_invoices`
- `job_input_materials`
- `job_tags`
- `job_attachments`
- `job_notes`
- `estimates`
- `estimate_options`
- `estimate_option_line_items`
- `estimate_option_notes`
- `estimate_option_attachments`
- `invoices`
- `invoice_items`
- `invoice_taxes`
- `invoice_discounts`
- `invoice_payments`
- `employees`
- `lead_sources`
- `tags`
- `events`
- `routes`
- `service_zones`
- `pipeline_statuses`
- `price_book_materials`
- `price_book_material_categories`
- `price_book_price_forms`
- `price_book_services`
- `activity_log`
- `daily_metrics`
- `raw_api_payloads`

Each resource table stores typed columns for common reporting fields and `raw_json` for future fields.

## Linear Status

Done:

- `ENG-254`: Create Go CLI skeleton for hcp.
- `ENG-255`: Add config and auth storage.
- `ENG-256`: Implement typed Housecall Pro API client.
- `ENG-257`: Add SQLite database, migrations, and typed store layer.
- `ENG-258`: Implement hcp sync into the local mirror.
- `ENG-259`: Implement basic list commands from local data.
- `ENG-260`: Implement owner brief commands.
- `ENG-261`: Implement funnel report.
- `ENG-262`: Implement sales report commands.
- `ENG-263`: Implement invoices and cash commands.
- `ENG-264`: Implement stale leads detection.
- `ENG-265`: Implement fulfillment report commands.
- `ENG-266`: Implement estimate follow-up detection.
- `ENG-267`: Implement stalled jobs detection.
- `ENG-268`: Implement funnel leakage report.
- `ENG-269`: Add report delivery framework.
- `ENG-270`: Add Slack delivery for owner and ops reports.
- `ENG-271`: Add email delivery for owner and ops reports.
- `ENG-272`: Implement MCP server shell for hcp.
- `ENG-273`: Add machine-readable Housecall Pro API catalog to hcp.
- `ENG-274`: Implement natural-language API planner coverage for all HCP endpoint families.
- `ENG-275`: Add guarded mutating API action framework.
- `ENG-276`: Implement full local mirror coverage for readable HCP resources.
- `ENG-277`: Add explicit agent-facing command recipes for every HCP API action.
- `ENG-279`: Add attachment and binary upload support for HCP API actions.
- `ENG-280`: Add webhook management and sync-watch integration.

Live credential validation:

- `ENG-253`: Done. API key auth is validated against `https://api.housecallpro.com`.
- The current key works with `Authorization: Token <key>`.
- Leave company id blank for this key. Sending the discovered company id as `X-Company-Id` returns `401 Unauthorized`.
- `GET /company` identifies the tenant as TruBlue of Chicagoland North.

Open finish-line issues for full English/API coverage:

- `ENG-278`: Done. Credentialed live validation covered read-only smoke, bounded sync, and controlled writes against temporary records created by the validation pass.

## Verification

Passing:

```bash
go test ./...
go build -o bin/hcp ./cmd/hcp
```

Live validation completed:

```bash
HCP_CONFIG=./.hcp-live-config.json ./bin/hcp auth doctor --endpoint /company --json
HCP_CONFIG=./.hcp-live-config.json ./bin/hcp api --method GET --path /pipeline/statuses --query resource_type=lead --query page_size=2 --json
HCP_CONFIG=./.hcp-live-config.json ./bin/hcp sync --db ./hcp-live-validation.sqlite --resource pipeline_statuses --page-size 2 --max-pages 1 --json
```

Live write validation records:

- Customer: `cus_8477dd36d9194ab3a8d4ff5727d4c8f5`
- Customer address: `adr_fdaf3e2a0036400c8884bd790f954705`
- Tag: `tag_e0a74489caee4b49bd988f228dae5311`
- Lead source: `lsrc_0af43c48873541dea8e13c506d786cc7`

Supported write paths validated:

- `POST /customers`
- `PUT /customers/{customer_id}`
- `POST /customers/{customer_id}/addresses`
- `POST /tags`
- `PUT /tags/{tag_id}`
- `POST /lead_sources`
- `PUT /lead_sources/{lead_source_id}`

Operational write hardening:

- `hcp api` now labels company schedule availability, company franchise info, pipeline statuses, app enable, dispatch, and schedule changes as `operational`.
- Operational writes require `--yes` plus the exact `--confirm <method:path>` token, like destructive writes.
- `hcp api` supports read-back verification with `--verify-get`, `--verify-query`, and `--verify-contains`.
- Schedule availability writes should not be considered complete unless `GET /company/schedule_availability` confirms the requested windows.

Branded interactive shell:

- `hcp crm` opens a Housecall Pro Command Center prompt.
- `hcp shell` remains as a developer-friendly alias for the same mode.
- The shell shows a branded ASCII banner plus config/auth/base URL status without exposing secrets.
- Inside the shell, normal commands run without the leading `hcp`.
- `status` maps to `auth doctor --endpoint /company`.
- `exit`, `quit`, `:q`, and `clear` are supported.
- Unknown natural-language-style lines route through `hcp api`.
- Unknown mutating lines default to `--plan`, not execution.
- `ai chatgpt`, `ai codex`, and `/ai chatgpt` explain the ChatGPT subscription path through Codex CLI.
- `ai providers` explains that OpenRouter, Anthropic, and OpenAI API-key embedded chat are separate future issues.

AI integration roadmap:

- `ENG-284`: ChatGPT subscription guidance through Codex in `hcp shell`.
- `ENG-285`: Embedded AI provider configuration for OpenRouter, Anthropic, and OpenAI API keys.
- `ENG-286`: Embedded AI chat loop with guarded `hcp` tool execution.

Cleanup:

- DELETE attempts against the temporary customer, address, tag, and lead source returned 404.
- The OpenAPI snapshot does not document delete endpoints for those resources, so cleanup is recorded as unsupported for this validation set.

Manual command checks performed:

```bash
./bin/hcp customers list --help
./bin/hcp customers list --db /tmp/hcp-empty-list-test-a.sqlite --json --limit 2
./bin/hcp customers list --db /tmp/hcp-empty-list-test-b.sqlite --limit 2
./bin/hcp customers list --data-source live --config /tmp/hcp-missing-config.json --limit 1
./bin/hcp api list customers --limit 10 --page 2 --plan --json
./bin/hcp api create customer --body '{"first_name":"Ada"}' --plan
HOUSECALL_PRO_API_KEY=dummy ./bin/hcp sync --since yesterday --dry-run --json
./bin/hcp brief today --db /tmp/hcp-empty-brief.sqlite --json
./bin/hcp funnel --db /tmp/hcp-empty-report.sqlite --json
./bin/hcp leads --uncontacted --db /tmp/hcp-empty-leads-root.sqlite --json
./bin/hcp estimates high-value --min 5000 --db /tmp/hcp-empty-est.sqlite --json
./bin/hcp invoices aging --db /tmp/hcp-empty-inv.sqlite --json
./bin/hcp cash collected --yesterday --db /tmp/hcp-empty-cash.sqlite --json
./bin/hcp report owner-daily --send slack --dry-run --json
./bin/hcp mcp serve --json
./bin/hcp api catalog --json
```

Expected live-mode missing-auth behavior is confirmed when no API key is configured.

## Known Gaps

Live API notes:

- The configured key is usable for reads and writes.
- Mutating validation should continue to use clearly named test records and clean them up only where the API supports deletion.
- Parent-data-dependent reads are limited by this tenant's current data. Jobs, estimates, invoices, routes, and material categories are empty, so nested job/estimate/material endpoints cannot all be read-validated against real parent IDs yet.

API/report depth:

- `hcp api` provides generic full-surface API access, but natural-language inference is intentionally conservative; use explicit `--method`/`--path` for endpoints or phrasing it cannot infer.
- Report commands are local-first MVP implementations. They expose the requested command surface and stable JSON, but metrics should be tightened after real HCP payload validation.

Sync:

- `--watch` now runs polling sync with `--watch`, `--watch-interval`, and `--watch-max-runs`.
- Appointment sync requires synced jobs first because HCP appointments are nested under job endpoints.

## Recommended Next Slice

No open Linear issues remain for this project goal. Future hardening should focus on adding richer typed helpers for high-volume workflows after real operator usage shows which API actions need first-class commands beyond the generic full-surface executor.
