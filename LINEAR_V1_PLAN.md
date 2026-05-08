# Linear Plan: HouseCallPro CLI V1

Client: Sandbox  
Project name: HouseCallPro CLI V1  
Working repo: `hcp-cli`  
Primary brief: ./Housecall_Pro_CLI_V1_Brief_Rozeta_Labs.md  
Implementation guide: ./PRINTING_PRESS_GUIDE.md

Linear project: https://linear.app/rozeta-labs/project/housecallpro-cli-v1-c9cf7cf595f5  
Linear team: Engineering  
Linear status: Backlog  
Created in Linear: 2026-05-07

## Current Linear Status

Last updated: 2026-05-08

- `ENG-253`: In Progress — docs/auth path researched; still needs actual Sandbox/client credential confirmation.
- `ENG-254`: Done — Go/Cobra CLI skeleton, root help/version, command groups, and tests are implemented.
- `ENG-255`: Done — config/auth storage and `auth` commands are implemented.
- `ENG-256`: In Progress — shared HCP HTTP client exists; typed resource methods and pagination hardening remain.
- `ENG-257`: In Progress — initial SQLite mirror exists for customers, leads, jobs, estimates, and invoices; remaining tables/read methods remain.
- `ENG-258`: In Progress — bounded `hcp sync` works for initial core resources; `--since`, inserted/updated counts, employees, and appointments remain.
- `ENG-259`: Done — basic resource list commands read from the local SQLite mirror by default, with `--data-source live` retained for direct API checks.

## Project Summary

Build `hcp`, an open-source command layer for home-service operators running Housecall Pro. V1 turns Housecall Pro data into local, queryable operating intelligence through a CLI first, then exposes the same command layer through MCP.

The project should not be framed as "AI automation." It should be framed as:

> A CLI and AI-agent interface for turning Housecall Pro data into daily operating intelligence.

## Linear Project Setup

Recommended Linear project:

- Name: `HouseCallPro CLI V1`
- Client/customer: `Sandbox`
- Priority: High
- State: Planned / Backlog
- Target date: TBD after API/auth feasibility is confirmed
- Lead: project owner
- Teams: engineering/product team responsible for Sandbox client work

Created Linear issues:

- `ENG-253`: Confirm Housecall Pro API access and sandbox credential path
- `ENG-254`: Create Go CLI skeleton for hcp
- `ENG-255`: Add config and auth storage
- `ENG-256`: Implement typed Housecall Pro API client
- `ENG-257`: Add SQLite database, migrations, and typed store layer
- `ENG-258`: Implement hcp sync into the local mirror
- `ENG-259`: Implement basic list commands from local data
- `ENG-260`: Implement owner brief commands
- `ENG-261`: Implement funnel report
- `ENG-262`: Implement sales report commands
- `ENG-263`: Implement invoices and cash commands
- `ENG-264`: Implement stale leads detection
- `ENG-265`: Implement fulfillment report commands
- `ENG-266`: Implement estimate follow-up detection
- `ENG-267`: Implement stalled jobs detection
- `ENG-268`: Implement funnel leakage report
- `ENG-269`: Add report delivery framework
- `ENG-270`: Add Slack delivery for owner and ops reports
- `ENG-271`: Add email delivery for owner and ops reports
- `ENG-272`: Implement MCP server shell for hcp

Recommended labels:

- `client:sandbox`
- `product:hcp-cli`
- `area:auth`
- `area:api`
- `area:db`
- `area:sync`
- `area:cli`
- `area:reports`
- `area:mcp`
- `area:automation`
- `type:foundation`
- `type:operator-report`
- `type:leakage-detection`

## Milestones

### Milestone 1: Foundation

Goal: prove auth, API connectivity, SQLite storage, sync, and basic CLI shape.

Exit criteria:

- `hcp` binary runs locally.
- Auth/config is implemented.
- Housecall Pro API client can make authenticated requests.
- SQLite database and migrations exist.
- `hcp sync` can persist at least one real resource type.
- Basic list commands work with human and JSON output.

### Milestone 2: Operator Reports

Goal: ship the first useful operator-facing commands from local data.

Exit criteria:

- `hcp brief today` produces an owner-facing summary.
- `hcp funnel --last 30d` computes funnel stages from local data.
- `hcp estimates unsold` works from synced estimates/jobs.
- `hcp jobs active` works from synced jobs/appointments.
- `hcp invoices open` works from synced invoices.

### Milestone 3: Leakage Detection

Goal: deliver the V1 "wow" commands that reveal revenue and fulfillment problems.

Exit criteria:

- `hcp leads stale` identifies uncontacted or unbooked leads.
- `hcp estimates no-followup` identifies open estimates without follow-up.
- `hcp jobs stalled` identifies stuck jobs.
- `hcp jobs completed-not-invoiced` identifies completed jobs missing invoices.
- `hcp funnel leakage` summarizes lost/delayed value across the pipeline.

### Milestone 4: Automation

Goal: enable scheduled reports and external delivery.

Exit criteria:

- Owner daily report command exists.
- Ops daily report command exists.
- Slack delivery supports dry run.
- Email delivery supports dry run.
- CSV export works for report/list commands where useful.

### Milestone 5: MCP Mode

Goal: expose the same command layer to agents.

Exit criteria:

- `hcp mcp serve` starts locally.
- MCP tools share the same auth/client/store/report logic as the CLI.
- Read-only commands are marked as safe.
- Mutating or delivery commands require explicit confirmation/dry-run behavior.

## Issue Map

### Epic: Project Foundation

#### Create Go CLI skeleton

Priority: High  
Labels: `type:foundation`, `area:cli`, `client:sandbox`

Description:
Create the initial `hcp` CLI application skeleton using the repo's chosen Go CLI stack. Prefer Go/Cobra unless a strong technical reason emerges to choose otherwise.

Acceptance criteria:

- `hcp --help` works.
- Root command has version/help output.
- Command groups exist for `auth`, `sync`, `customers`, `jobs`, `estimates`, `leads`, `invoices`, `brief`, `funnel`, `tech`, `marketing`, and `mcp`.
- Basic command tests cover help output and command registration.

#### Add config and auth storage

Priority: High  
Labels: `type:foundation`, `area:auth`, `client:sandbox`

Description:
Implement local configuration for Housecall Pro API credentials and CLI settings.

Acceptance criteria:

- `hcp auth login` stores credentials/config securely enough for V1 local development.
- `hcp auth doctor` validates required config.
- Missing auth errors are clear and actionable.
- Non-interactive mode fails cleanly when auth is missing.

#### Implement Housecall Pro API client

Priority: High  
Labels: `type:foundation`, `area:api`, `client:sandbox`

Description:
Create a typed API client for Housecall Pro resources required by Sprint 1.

Acceptance criteria:

- Client supports base URL configuration.
- Auth headers are injected consistently.
- Pagination shape is handled or explicitly stubbed with TODOs where API details are unknown.
- API errors are translated into typed CLI errors.
- Initial resource methods exist for customers, jobs, estimates, leads, employees, invoices, and appointments.

#### Add SQLite database and migrations

Priority: High  
Labels: `type:foundation`, `area:db`, `client:sandbox`

Description:
Create the local data mirror using SQLite and migrations.

Acceptance criteria:

- DB path is configurable.
- Migrations run idempotently.
- Tables exist for customers, leads, jobs, estimates, appointments, employees, invoices, job_notes, estimate_options, lead_sources, events, activity_log, and daily_metrics.
- Store layer exposes typed upsert/read methods for Sprint 1 resources.

#### Implement `hcp sync`

Priority: High  
Labels: `type:foundation`, `area:sync`, `client:sandbox`

Description:
Implement sync from Housecall Pro into the local SQLite mirror.

Acceptance criteria:

- `hcp sync` runs all enabled resource syncs.
- `hcp sync --since yesterday` parses a relative time window.
- Sync writes through typed store methods.
- Sync summary reports inserted/updated counts.
- Failed resource syncs surface actionable errors.

#### Implement basic list commands

Priority: High  
Labels: `type:foundation`, `area:cli`, `client:sandbox`

Description:
Implement baseline list commands for grounding and debugging.

Acceptance criteria:

- `hcp customers list`
- `hcp jobs list`
- `hcp estimates list`
- `hcp leads list`
- Commands support `--json`, `--limit`, and bounded default output.
- Commands can read from local data after `hcp sync`.

### Epic: Operator Reports

#### Implement owner brief command

Priority: High  
Labels: `type:operator-report`, `area:reports`, `client:sandbox`

Description:
Build `hcp brief today`, `hcp brief yesterday`, `hcp brief week`, and `hcp brief month`.

Acceptance criteria:

- Brief includes new leads, booked appointments, estimates created/sold, revenue booked/completed, unsold estimate value, stale opportunity value, jobs completed/delayed, unpaid invoices, top techs, and marketing source performance where data exists.
- Command supports `--json`.
- Output includes data freshness timestamp.

#### Implement funnel report

Priority: High  
Labels: `type:operator-report`, `area:reports`, `client:sandbox`

Description:
Build funnel reporting for Lead -> Appointment -> Estimate -> Sold Job -> Completed Job -> Paid Invoice.

Acceptance criteria:

- `hcp funnel --last 30d` works.
- Supports source and employee filters if data relationships are available.
- Reports conversion rates, sold revenue, completed revenue, paid revenue, average sales cycle, and average ticket.
- JSON output is stable and documented in tests.

#### Implement sales report commands

Priority: Normal  
Labels: `type:operator-report`, `area:reports`, `client:sandbox`

Description:
Implement initial sales-oriented commands for unsold and open estimates.

Acceptance criteria:

- `hcp estimates open`
- `hcp estimates unsold --last 30d`
- `hcp estimates stale --days 3`
- `hcp estimates high-value --min 5000`
- Commands support `--json`, `--limit`, and meaningful sort order.

#### Implement fulfillment report commands

Priority: Normal  
Labels: `type:operator-report`, `area:reports`, `client:sandbox`

Description:
Implement initial fulfillment commands for active and schedule-sensitive jobs.

Acceptance criteria:

- `hcp jobs today`
- `hcp jobs tomorrow`
- `hcp jobs active`
- `hcp jobs unscheduled`
- `hcp jobs delayed`
- Commands support `--json`.

#### Implement invoices and cash commands

Priority: Normal  
Labels: `type:operator-report`, `area:reports`, `client:sandbox`

Description:
Implement open invoice and cash collection reporting.

Acceptance criteria:

- `hcp invoices open`
- `hcp invoices overdue`
- `hcp invoices aging`
- `hcp cash collected --yesterday`
- Commands support `--json`.

### Epic: Leakage Detection

#### Implement stale leads detection

Priority: High  
Labels: `type:leakage-detection`, `area:reports`, `client:sandbox`

Description:
Build stale lead detection for uncontacted or unbooked leads.

Acceptance criteria:

- `hcp leads stale --minutes 15` works.
- `hcp leads --uncontacted` works.
- `hcp leads --unbooked` works.
- Output shows lead age, source, contact status, booking status, and recommended next action.

#### Implement estimate follow-up detection

Priority: High  
Labels: `type:leakage-detection`, `area:reports`, `client:sandbox`

Description:
Identify open estimates with no recent follow-up activity.

Acceptance criteria:

- `hcp estimates no-followup` works.
- Configurable stale threshold exists.
- Output includes estimate value, owner/employee, age, last activity, and customer.

#### Implement stalled jobs detection

Priority: High  
Labels: `type:leakage-detection`, `area:reports`, `client:sandbox`

Description:
Identify sold, scheduled, active, or completed jobs that are stuck relative to expected lifecycle.

Acceptance criteria:

- `hcp jobs stalled` works.
- `hcp jobs completed-not-invoiced` works.
- Output includes stall reason and value/revenue impact when known.

#### Implement funnel leakage

Priority: High  
Labels: `type:leakage-detection`, `area:reports`, `client:sandbox`

Description:
Summarize where revenue or opportunities are leaking across the lifecycle.

Acceptance criteria:

- `hcp funnel leakage` works.
- Reports leads without appointments, appointments without estimates, unsold estimate value, sold jobs not scheduled, and completed jobs unpaid.
- Output has both human-readable summary and `--json`.

### Epic: Automation

#### Add report delivery framework

Priority: Normal  
Labels: `area:automation`, `client:sandbox`

Description:
Create shared delivery plumbing for future Slack, email, and CSV export.

Acceptance criteria:

- Delivery commands support `--dry-run`.
- Failures are explicit and do not mark report delivery successful.
- Report payloads can be rendered as text and JSON.

#### Add Slack delivery

Priority: Normal  
Labels: `area:automation`, `client:sandbox`

Description:
Implement Slack delivery for owner and ops reports.

Acceptance criteria:

- `hcp report owner-daily --send slack --dry-run` renders payload locally.
- Slack credentials/config are validated.
- Actual send path is gated by explicit non-dry-run execution.

#### Add email delivery

Priority: Low  
Labels: `area:automation`, `client:sandbox`

Description:
Implement email delivery for owner and ops reports.

Acceptance criteria:

- `hcp report owner-daily --send email --dry-run` renders payload locally.
- Email provider config is validated.
- Actual send path is gated by explicit non-dry-run execution.

### Epic: MCP

#### Implement MCP server shell

Priority: Normal  
Labels: `area:mcp`, `client:sandbox`

Description:
Expose the CLI's shared command/report layer as an MCP server.

Acceptance criteria:

- `hcp mcp serve` starts locally.
- Initial read-only tools exist for brief, funnel, stale leads, unsold estimates, stalled jobs, and open invoices.
- Tools reuse existing store/report code.
- Mutating or delivery actions are excluded or explicitly guarded.

## Initial Dependency Order

1. Create Go CLI skeleton
2. Add config and auth storage
3. Implement Housecall Pro API client
4. Add SQLite database and migrations
5. Implement `hcp sync`
6. Implement basic list commands
7. Implement owner brief command
8. Implement funnel report
9. Implement leakage detection commands
10. Implement delivery framework
11. Implement MCP server shell

## Open Questions for Linear Setup

- Which Linear team should own the project?
- Does the workspace already have a `Sandbox` customer/client record, label, initiative, or project grouping?
- Should V1 have a target launch date now, or wait until Housecall Pro API/auth feasibility is confirmed?
- Who should be project lead and default assignee for foundation issues?
- Should we use Linear milestones for the five sprints, or create parent epic issues instead?
