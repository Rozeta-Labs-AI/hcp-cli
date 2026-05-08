# Housecall Pro API Coverage Gap Analysis

Generated: 2026-05-08

## Goal

Make `hcp` usable by Codex or Claude Code as an English-language command layer for every Housecall Pro API action.

## Source Inventory

Local source files:

- `HOUSECALL_PRO_API_REFERENCE.md`
- `HOUSECALL_PRO_OPENAPI_SNAPSHOT.yaml`

The OpenAPI snapshot currently contains:

- 71 paths
- 95 operations
- Read operations for application, checklists, customers, employees, jobs, appointments, invoices, estimates, company, schedule, events, tags, leads, lead sources, job types, price book, service zones, routes, and pipeline statuses.
- Mutating operations for application enable/disable, customers, customer addresses, jobs, job line items, appointments, schedules, dispatch, job tags/notes/links/locks, webhooks, estimates/options, company franchise/schedule settings, tags, leads/conversion, lead sources, job types, price book materials/categories/forms, and pipeline statuses.

The CLI now exposes this inventory through:

```bash
hcp api catalog --json
hcp api catalog --area jobs
```

## Current Coverage

Implemented:

- API key/OAuth auth plumbing.
- Generic raw JSON API executor: `hcp api --method <METHOD> --path <PATH>`.
- Natural-language planner for all major endpoint families, with `--param key=value` substitution for required path params.
- Machine-readable API catalog from the OpenAPI snapshot.
- Guarded mutating action framework with `--plan`, `--dry-run`, `--yes`, risk labels, and additional confirmation for destructive actions.
- Local-first operator/report command surface from the original Linear project.
- SQLite mirror and bounded sync for core report resources plus readable endpoint families needed for agent context.

## Linear Cross-Reference

Original issues now done:

- `ENG-253` through `ENG-272`.

Live credential findings:

- Base URL is `https://api.housecallpro.com`.
- Auth works with `Authorization: Token <key>`.
- Leave company id blank for the current key. Sending `X-Company-Id` with the discovered company id returns `401 Unauthorized`.
- The current tenant is a live-test path, not a separate sandbox.

New finish-line issues added:

- `ENG-273`: Add machine-readable Housecall Pro API catalog to hcp.
- `ENG-274`: Implement natural-language API planner coverage for all HCP endpoint families. Done locally.
- `ENG-275`: Add guarded mutating API action framework.
- `ENG-276`: Implement full local mirror coverage for readable HCP resources.
- `ENG-277`: Add explicit agent-facing command recipes for every HCP API action.
- `ENG-278`: Implement credentialed live API validation suite.
- `ENG-279`: Add attachment and binary upload support for HCP API actions.
- `ENG-280`: Add webhook management and sync-watch integration.

## Remaining Finish-Line Work

None currently logged in Linear. Live write validation was run only against temporary records created during the validation pass. Housecall Pro returned 404 for cleanup DELETE attempts on those temporary customer, address, tag, and lead-source records; the OpenAPI snapshot does not document delete operations for those resources, so cleanup is recorded as unsupported by the tested API surface.

## Verification

Passing locally:

```bash
go test ./...
go build -o bin/hcp ./cmd/hcp
./bin/hcp api catalog --json
```

Live read-only validation completed against the configured tenant:

- `hcp auth doctor --endpoint /company`
- direct GET smoke checks for company, employees, customers, leads, jobs, estimates, invoices, lead sources, tags, events, service zones, routes, price book categories/forms/services, schedule availability, booking windows, and pipeline statuses with `resource_type=lead`
- bounded sync into `hcp-live-validation.sqlite` for companies, employees, customers, customer addresses, lead sources, leads, lead line items, jobs, estimates, invoices, tags, events, service zones, routes, price book services, and pipeline statuses

Known required-filter endpoints:

- `/pipeline/statuses` requires `resource_type`.
- `/checklists` requires job or estimate UUID filters.
- `/api/price_book/materials` requires `material_category_uuid`.

Live write validation completed against temporary records:

- Created and updated customer `cus_8477dd36d9194ab3a8d4ff5727d4c8f5`.
- Created customer address `adr_fdaf3e2a0036400c8884bd790f954705` under that customer.
- Created and updated tag `tag_e0a74489caee4b49bd988f228dae5311`.
- Created and updated lead source `lsrc_0af43c48873541dea8e13c506d786cc7`.
- Cleanup DELETE attempts for those temporary resources returned 404, matching the lack of documented delete endpoints in the OpenAPI snapshot.
