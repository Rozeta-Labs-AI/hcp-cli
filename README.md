# Housecall Pro CLI (`hcp`)

`hcp` is a command-line interface for Housecall Pro built for operators, developers, and AI coding agents. It provides authenticated access to the Housecall Pro API, a local SQLite mirror for reporting, and a generic API action layer that lets tools like Codex or Claude Code safely perform Housecall Pro actions from natural-language instructions.

The project currently exposes the full documented API surface from the included Housecall Pro OpenAPI snapshot:

- 95 documented API operations through `hcp api catalog`.
- Generic JSON API execution with explicit `--method` and `--path`.
- Natural-language endpoint planning for common Housecall Pro resource families.
- Guardrails for write actions with `--plan`, `--dry-run`, `--yes`, operational/destructive confirmation tokens, and optional read-back verification.
- Multipart/file upload support for attachment endpoints.
- Local SQLite sync for reporting and agent context.
- Report commands for owner briefs, funnel, leads, estimates, jobs, invoices, cash, and delivery dry-runs.
- MCP server shell for read-only report/tool access.

## What This Accomplishes

Housecall Pro has a broad API. This CLI gives a human or AI agent a single local tool that can:

- Discover available API operations.
- Authenticate against a user's own Housecall Pro API key.
- Run read-only API requests.
- Plan write actions before executing them.
- Execute supported create/update/delete actions with explicit confirmation.
- Sync Housecall Pro data into a local SQLite database.
- Query local data for reports and operational summaries.
- Use explicit method/path fallback for any API operation the natural-language planner cannot infer.

The intended agent workflow is:

1. Use `hcp api catalog --json` to discover the exact API action.
2. Use `hcp api examples --json` for command recipes.
3. Use `--plan` for any write action.
4. Execute only after the user explicitly requests the action.

## Install For End Users

### Option 1: Install From Source With Go

Prerequisites:

- Go 1.22 or newer.
- Access to this private GitHub repository.
- A Housecall Pro API key for the user's own account.

Install:

```bash
go install github.com/Rozeta-Labs-AI/hcp-cli/cmd/hcp@latest
```

Make sure Go's binary directory is on your `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Verify:

```bash
hcp --help
```

For private repository installs, your machine must be authenticated with GitHub. A reliable setup is:

```bash
gh auth login
go env -w GOPRIVATE=github.com/Rozeta-Labs-AI/*
go install github.com/Rozeta-Labs-AI/hcp-cli/cmd/hcp@latest
```

### Option 2: Build From A Local Clone

```bash
git clone https://github.com/Rozeta-Labs-AI/hcp-cli.git
cd hcp-cli
go build -o bin/hcp ./cmd/hcp
```

Run it directly:

```bash
./bin/hcp --help
```

Or install it into a directory on your `PATH`:

```bash
cp ./bin/hcp /usr/local/bin/hcp
hcp --help
```

### Option 3: Install From GitHub Releases

When release binaries are published, download the archive for your operating system and CPU architecture from the repository's Releases page.

Typical macOS/Linux install:

```bash
tar -xzf hcp_<version>_<os>_<arch>.tar.gz
chmod +x hcp
mv hcp /usr/local/bin/hcp
hcp --help
```

Typical Windows install:

1. Download the Windows ZIP archive.
2. Extract `hcp.exe`.
3. Move `hcp.exe` into a folder on your `PATH`.
4. Open PowerShell and run:

```powershell
hcp --help
```

## Connect Your Own Housecall Pro Account

Each user configures their own credentials locally. Credentials are not shared through the repository.

### Store Credentials Locally

```bash
hcp auth login --api-key <your-housecall-pro-api-key>
```

Then validate:

```bash
hcp auth doctor
```

To validate against the company endpoint:

```bash
hcp auth doctor --endpoint /company
```

### Use Environment Variables Instead

If you do not want to store credentials in the local config file:

```bash
export HOUSECALL_PRO_API_KEY=<your-housecall-pro-api-key>
export HCP_BASE_URL=https://api.housecallpro.com
hcp auth doctor
```

### Company ID Note

Leave company id blank unless your Housecall Pro API key specifically requires it. In live validation for this project, the API key worked without `X-Company-Id`, and sending the discovered company id in that header returned `401 Unauthorized`.

## Quick Start

Check auth:

```bash
hcp auth status
hcp auth doctor
```

List available API operations:

```bash
hcp api catalog
hcp api catalog --json
hcp api catalog --area jobs
```

List command recipes:

```bash
hcp api examples
hcp api examples --json
hcp api examples --area customers
```

Run read-only API calls:

```bash
hcp api get /company --json
hcp api list customers --limit 25 --json
hcp api --method GET --path /pipeline/statuses --query resource_type=lead --json
```

Plan a write action:

```bash
hcp api create customer --body '{"first_name":"Ada","last_name":"Lovelace"}' --plan
```

Execute a write action only after reviewing the plan:

```bash
hcp api create customer --body '{"first_name":"Ada","last_name":"Lovelace"}' --yes
```

Destructive actions require an additional confirmation token:

```bash
hcp api --method DELETE --path /api/price_book/price_forms/form_uuid --plan
hcp api --method DELETE --path /api/price_book/price_forms/form_uuid --yes --confirm delete:/api/price_book/price_forms/form_uuid
```

## Branded Interactive Shell

For a more dedicated command-center experience, open the branded CRM shell:

```bash
hcp crm
```

`hcp shell` is kept as a developer-friendly alias for the same experience.

You will see the Housecall Pro Command Center banner, local config/auth status, and an `hcp>` prompt.

On first interactive use, `hcp crm` offers AI Assistant Setup:

```text
AI Assistant Setup

Choose how you want hcp crm to think:
  1. ChatGPT subscription via Codex
  2. OpenRouter API key
  3. Anthropic API key
  4. OpenAI API key
  5. Ollama local model
  6. Skip for now
```

You can run the setup picker any time:

```bash
hcp setup model
```

Or inside the CRM shell:

```text
hcp> setup model
hcp> ai status
```

Inside the shell, run normal commands without typing the leading `hcp`:

```text
hcp> status
hcp> api get /company --json
hcp> api list customers --limit 5 --json
hcp> sync --resource customers --resource leads --json
hcp> customers list --limit 5 --json
hcp> exit
```

The shell also accepts simple natural-language-style API lines. Unknown mutating requests default to a plan instead of execution:

```text
hcp> list customers --limit 5 --json
hcp> create lead source --body '{"name":"Spring Mailer"}'
```

To execute a planned mutating action, run the explicit `api ... --yes` command after reviewing the plan.

### ChatGPT Subscription Through Codex

ChatGPT Plus/Pro subscription access is not configured in `hcp` as a raw API key. Use Codex CLI as the AI layer, then have Codex operate `hcp` as the local Housecall Pro tool:

```text
ChatGPT Plus/Pro -> Codex CLI -> hcp CLI -> Housecall Pro API
```

Inside `hcp crm` or `hcp shell`, run:

```text
hcp> ai chatgpt
```

That prints setup instructions and safe starter prompts. The short version:

```bash
npm install -g @openai/codex
codex --login
# choose Sign in with ChatGPT
```

Then ask Codex:

```text
Use the hcp CLI to verify auth and list my first 5 Housecall Pro customers as JSON. Do not modify anything.
```

OpenRouter, Anthropic, and OpenAI API-key based embedded shell chat are tracked separately from the ChatGPT subscription path.

## Natural-Language And Explicit API Actions

The planner supports common English-style commands:

```bash
hcp api list customers --json
hcp api create lead --body '{"customer_id":"cus_123"}' --plan
hcp api update customer cus_123 --body '{"last_name":"Updated"}' --plan
```

For exact control, use method/path:

```bash
hcp api --method GET --path /customers --query page_size=25 --json
hcp api --method POST --path /lead_sources --body '{"name":"Google Ads"}' --plan
hcp api --method PUT --path /tags/tag_123 --body '{"name":"VIP"}' --plan
```

Use `--param` for paths with placeholders inferred by the planner:

```bash
hcp api add customer address --param customer_id=cus_123 --body '{"street":"1 Main St","city":"Chicago","state":"IL","zip":"60601","country":"US"}' --plan
```

## Required Query Parameters Found During Live Validation

Some Housecall Pro endpoints require filters:

```bash
hcp api --method GET --path /pipeline/statuses --query resource_type=lead --json
hcp api --method GET --path /checklists --query job_uuids[]=job_123 --json
hcp api --method GET --path /api/price_book/materials --query material_category_uuid=cat_uuid --json
```

## File Uploads

Use multipart support for attachment endpoints:

```bash
hcp api add job attachment \
  --param job_id=job_123 \
  --file ./photo.jpg \
  --file-field attachment \
  --content-type image/jpeg \
  --body '{"description":"Before photo"}' \
  --plan
```

## Sync To Local SQLite

Run a bounded sync into the default local database:

```bash
hcp sync --resource customers --resource jobs --resource estimates --json
```

Use a custom database path:

```bash
hcp sync --db ./hcp.sqlite --resource customers --resource leads --json
```

Limit sync scope:

```bash
hcp sync --page-size 50 --max-pages 2 --json
```

Polling watch mode:

```bash
hcp sync --watch --watch-interval 5m --watch-max-runs 12 --json
```

## Local Report Commands

Examples:

```bash
hcp brief today --json
hcp funnel --last 30d --json
hcp leads stale --minutes 15 --json
hcp estimates open --json
hcp jobs stalled --json
hcp invoices aging --json
hcp cash collected --yesterday --json
```

Reports read from the local SQLite mirror. Run `hcp sync` first when you need fresh data.

## Configuration

Common environment variables:

| Variable | Purpose |
| --- | --- |
| `HOUSECALL_PRO_API_KEY` | Housecall Pro API key |
| `HCP_BASE_URL` | API base URL, defaults to `https://api.housecallpro.com` |
| `HCP_CONFIG` | Override config file path |
| `HCP_DB` | Override SQLite database path |

Local config is stored under the operating system's user config directory unless overridden with `--config` or `HCP_CONFIG`.

Local SQLite data is stored under the operating system's user cache directory unless overridden with `--db` or `HCP_DB`.

## Safety Model

The CLI is designed for AI-agent use, so write operations are guarded:

- `GET` requests can run directly.
- `POST`, `PUT`, `PATCH`, and `DELETE` require `--plan`, `--dry-run`, or `--yes`.
- Operational and destructive actions require `--yes` plus an exact `--confirm <method:path>` token.
- Plans show method, path, query, body, mutability, risk label, and files.
- Use `--verify-get` and `--verify-contains` for writes where Housecall Pro may return a success-shaped response before the setting actually persists.

Operational writes include company schedule availability, company franchise info, pipeline statuses, app enable, dispatch, and job or estimate schedule changes.

Example read-back verification:

```bash
hcp api --method PUT --path /company/schedule_availability \
  --body '{"daily_schedule_windows":[...]}' \
  --plan

hcp api --method PUT --path /company/schedule_availability \
  --body '{"daily_schedule_windows":[...]}' \
  --yes \
  --confirm put:/company/schedule_availability \
  --verify-get /company/schedule_availability \
  --verify-contains '"start_time":"09:00"' \
  --verify-contains '"end_time":"17:00"'
```

If verification fails, do not treat the write as complete. Use the Housecall Pro UI or a stricter full-payload API request and verify again.

Recommended rule for agents:

Never execute a mutating command unless the user explicitly asked for that specific action.

## Development

Run tests:

```bash
go test ./...
```

Build:

```bash
go build -o bin/hcp ./cmd/hcp
```

Validate the API catalog:

```bash
./bin/hcp api catalog --json
```

## Repository Notes

Do not commit:

- `.env`
- local config files such as `.hcp-live-config.json`
- SQLite databases
- built binaries
- API keys or customer data

The repository includes project documentation and a local OpenAPI snapshot so the CLI can expose a machine-readable API catalog without fetching docs at runtime.
