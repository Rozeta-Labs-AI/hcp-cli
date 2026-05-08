# Printing Press Guide for `hcp`

This project is being built in the spirit of Printing Press:

- Website: https://printingpress.dev/
- Source: https://github.com/mvanhorn/cli-printing-press
- Local product brief: ./Housecall_Pro_CLI_V1_Brief_Rozeta_Labs.md

## Core Takeaway

Printing Press is not just a generator for endpoint wrappers. Its main idea is that the best agent-facing CLIs are domain-shaped command layers over a local data mirror.

For `hcp`, that means we should not think of Housecall Pro as "customers, jobs, estimates, invoices." We should think of it as an operating system for a home-service business:

> Housecall Pro is not just a field-service CRM. It is the operational ledger of leads, sales, fulfillment, technician performance, dispatch risk, and cash collection.

That is the non-obvious identity of the API. Every lead, estimate, appointment, job status, invoice, note, photo, and employee assignment is a signal about where revenue is being created, delayed, lost, or collected.

## How This Should Shape `hcp`

The CLI should have three layers.

### 1. API Completeness

We still need direct resource access for grounding, debugging, and completeness:

```bash
hcp customers list
hcp jobs list
hcp estimates list
hcp leads list
hcp invoices list
```

These are the baseline. They prove auth, pagination, API coverage, and terminal output.

But they are not the product.

### 2. Local Mirror

Printing Press treats local SQLite as the foundation for serious CLI behavior. `hcp` should do the same.

The local mirror should use real typed tables, not generic JSON blobs:

- `customers`
- `leads`
- `jobs`
- `estimates`
- `appointments`
- `employees`
- `invoices`
- `job_notes`
- `estimate_options`
- `lead_sources`
- `events`
- `activity_log`
- `daily_metrics`

The sync layer should support:

```bash
hcp sync
hcp sync --since yesterday
hcp sync --watch
```

It should eventually support cursor tracking, incremental sync, batch transactions, and safe refresh behavior before read-heavy commands.

### 3. Operator Commands

The winning commands are compound workflows that the raw API does not expose:

```bash
hcp brief today
hcp leads stale --minutes 15
hcp funnel leakage
hcp estimates no-followup
hcp jobs stalled
hcp fulfillment bottlenecks
hcp tech scorecard --last 30d
hcp marketing sources --last 30d
```

These commands should mostly read from the local store. They should join across resources and produce answers an owner or operations manager can act on immediately.

## Agent-Native CLI Rules

Printing Press is explicit that the CLI should be designed for agents first, which usually makes it better for humans too.

Every command should be predictable under automation:

- `--json` for structured output.
- `--csv` where tabular export matters.
- `--compact` for token-efficient output.
- `--select` for choosing fields.
- `--limit` on list/report commands.
- `--dry-run` on anything that could mutate or send.
- `--quiet` for scripts.
- `--no-input` for non-interactive runs.
- `--yes` for explicitly approved actions.

Output should be bounded by default. If a command can return a lot of rows, it should show a limit and tell the user how to narrow the result.

Errors should be actionable. A failed command should say which argument, flag, auth value, or sync state is missing.

Typed exit codes should be consistent:

- `0`: success
- `2`: usage error
- `3`: not found
- `4`: auth/config error
- `5`: API error
- `7`: rate limited

## Data Freshness Model

The CLI should make freshness explicit:

```bash
hcp jobs active --data-source local
hcp jobs active --data-source live
hcp jobs active --data-source auto
```

Recommended semantics:

- `local`: only read SQLite.
- `live`: call Housecall Pro directly and do not depend on local cache.
- `auto`: perform a bounded pre-read refresh when the command has a known sync path, then read locally.

V1 can start with manual `hcp sync`, but the architecture should leave room for this model.

## MCP Strategy

Printing Press generates both a CLI and MCP server from the same internal client/store layer.

For `hcp`, the future target is:

```bash
hcp mcp serve
```

The MCP surface should expose the same meaningful operator commands, not just raw endpoints. Commands that only read from local data or Housecall Pro should be marked read-only when implemented. Commands that send Slack/email, create notes, update jobs, or mutate external state should require explicit permission and support `--dry-run`.

## Verification Bar

The project should adopt a Printing Press-style quality gate before treating a feature as done.

For each command, verify:

- The API paths it uses are real.
- Auth headers match Housecall Pro's expected auth scheme.
- Every flag registered by the command is actually used.
- Store-backed commands have a write path from sync and a read path from the command.
- List/report commands support JSON output and bounded results.
- Mutating or delivery commands support `--dry-run`.
- The command works in non-interactive mode.

For local data features, the key behavioral proof is:

> Does `hcp sync` write the rows that the report/search/leakage command reads?

If not, the command is not real yet.

## Build Order for This Repo

Follow the HCP brief sprint order, but apply the Printing Press hierarchy.

### Sprint 1: Foundation

Goal: prove the shell, auth, API client, SQLite store, and sync path.

Commands:

```bash
hcp auth login
hcp auth doctor
hcp sync
hcp customers list
hcp jobs list
hcp estimates list
hcp leads list
```

Implementation priorities:

- Choose the Go CLI stack unless there is a strong reason not to. Printing Press uses Go/Cobra, and the generated ecosystem assumes that style.
- Use a real config/auth layer.
- Use SQLite first.
- Add migrations early.
- Keep table schemas domain-specific.
- Make JSON output a first-class path from the start.

### Sprint 2: Operator Reports

Goal: turn synced data into operator intelligence.

Commands:

```bash
hcp brief today
hcp funnel --last 30d
hcp estimates unsold
hcp jobs active
hcp invoices open
```

These should be SQLite-backed wherever possible.

### Sprint 3: Leakage Detection

Goal: ship the "why this CLI exists" commands.

Commands:

```bash
hcp leads stale
hcp estimates no-followup
hcp jobs stalled
hcp jobs completed-not-invoiced
hcp funnel leakage
```

This is where the local mirror starts beating a dashboard or endpoint wrapper.

### Sprint 4: Automation

Goal: delivery and scheduled reporting.

Commands:

```bash
hcp report owner-daily --send email
hcp report ops-daily --send slack
hcp alert stale-leads --minutes 10
hcp alert high-value-estimate --min 10000
```

Anything that sends externally must support `--dry-run`.

### Sprint 5: MCP

Goal: expose the same command layer to agents.

Command:

```bash
hcp mcp serve
```

The MCP server should share the same client, store, auth, and report logic as the CLI.

## Design Principle

Do not build a thin Housecall Pro wrapper.

Build the operator-native command layer:

```text
raw API resources -> local operational mirror -> compound business insight commands -> CLI + MCP surfaces
```

When choosing between two implementation options, prefer the one that makes future commands like `funnel leakage`, `jobs stalled`, `brief today`, and `tech scorecard` easier to compute reliably from local data.

