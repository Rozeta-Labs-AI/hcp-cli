# Agent Guide: Using `hcp` for Housecall Pro API Actions

This guide is for Codex, Claude Code, and other coding agents operating the local `hcp` CLI.

## Rules

- Prefer `--json` when another program will read the output.
- Use `--plan` or `--dry-run` before any mutating action.
- Do not execute mutating actions unless the user explicitly asked for the action.
- Use explicit `--method` and `--path` when natural language is ambiguous.
- Operational and destructive actions need both `--yes` and the exact `--confirm <method:path>` token shown by the plan.
- For operational settings, use read-back verification and do not claim success unless the verification passes.

## Discover API Actions

List every documented Housecall Pro operation:

```bash
hcp api catalog --json
```

Filter by area:

```bash
hcp api catalog --area jobs --json
hcp api catalog --area price_book --json
```

List command recipes:

```bash
hcp api examples --json
hcp api examples --area estimates --json
```

## Interactive Shell

Use `hcp crm` when the user wants a branded command-center experience. `hcp shell` is the developer-friendly alias for the same mode. Inside the shell, omit the leading `hcp`:

```text
hcp> status
hcp> api get /company --json
hcp> api list customers --limit 5 --json
hcp> create lead source --body '{"name":"Spring Mailer"}'
```

Unknown mutating shell lines default to a plan. Do not execute the follow-up `api ... --yes` form unless the user explicitly confirms the action.

### ChatGPT Subscription Mode

ChatGPT Plus/Pro is supported through Codex CLI, not as an embedded `hcp` API provider.

```text
hcp> ai chatgpt
```

This explains:

```text
ChatGPT Plus/Pro -> Codex CLI -> hcp CLI -> Housecall Pro API
```

Use API-provider integrations such as OpenRouter, Anthropic, or OpenAI API only for future embedded shell chat. Do not imply that ChatGPT subscription credentials can be pasted into `hcp` as an API key.

## Read Actions

Examples:

```bash
hcp api list customers --limit 25 --json
hcp api get /company --json
hcp api --method GET --path /jobs/job_123/appointments --json
hcp api --method GET --path /pipeline/statuses --query resource_type=lead --json
hcp api --method GET --path /api/price_book/materials --query material_category_uuid=cat_uuid --json
hcp api --method GET --path /checklists --query job_uuids[]=job_123 --json
```

## Mutating Actions

Plan first:

```bash
hcp api create customer --body '{"first_name":"Ada","last_name":"Lovelace"}' --plan
hcp api --method PUT --path /jobs/job_123/schedule --body '{"scheduled_start":"2026-05-09T09:00:00Z"}' --plan
```

Execute only after explicit user intent:

```bash
hcp api create customer --body '{"first_name":"Ada","last_name":"Lovelace"}' --yes
```

Destructive example:

```bash
hcp api --method DELETE --path /jobs/job_123/schedule --plan
hcp api --method DELETE --path /jobs/job_123/schedule --yes --confirm delete:/jobs/job_123/schedule
```

Operational write with verification:

```bash
hcp api --method PUT --path /company/schedule_availability --body '{"daily_schedule_windows":[]}' --plan
hcp api --method PUT --path /company/schedule_availability --body '{"daily_schedule_windows":[]}' --yes --confirm put:/company/schedule_availability --verify-get /company/schedule_availability --verify-contains '"daily_schedule_windows"'
```

## Full-Surface Fallback

If a plain-English phrase does not infer the exact endpoint, use the catalog and call the endpoint explicitly:

```bash
hcp api --method POST --path /leads/lead_123/convert --body '{"convert_to":"job"}' --plan
hcp api --method PUT --path /pipeline/statuses --body '{"resource_type":"lead","statuses":[]}' --plan
hcp api --method DELETE --path /api/price_book/materials/mat_uuid --plan
```

## Live API Notes

- Leave company id blank for the current API key. `GET /company` works without `X-Company-Id`; sending the discovered company id in that header returns `401 Unauthorized`.
- `/pipeline/statuses` requires `resource_type` with `lead`, `job`, or `estimate`.
- `/api/price_book/materials` requires `material_category_uuid`.
- `/checklists` requires one or more job or estimate UUID filters.
- Schedule availability writes may return a success-shaped response without persisting the requested change. Always read back with `--verify-get /company/schedule_availability` and verify the expected windows.

## File Uploads

Use `--file` for endpoints that require multipart uploads:

```bash
hcp api add job attachment --param job_id=job_123 --file ./photo.jpg --file-field attachment --content-type image/jpeg --body '{"description":"Before photo"}' --plan
hcp api --method POST --path /estimates/est_123/options/opt_123/attachments --file ./proposal.pdf --file-field attachment --content-type application/pdf --plan
```

## Credential Validation

Before live API work:

```bash
hcp auth status
hcp auth doctor
```

If no API key is configured, use:

```bash
hcp auth login --api-key <housecall-pro-api-key>
```
