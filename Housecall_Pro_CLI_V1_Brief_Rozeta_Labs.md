# Housecall Pro CLI V1 Brief

## Core thesis

Home service companies care about three things:

1. Leads  
2. Sales  
3. Fulfillment  

The CLI should turn Housecall Pro into an operator command center.

Instead of logging into dashboards, exporting reports, or asking someone to “pull the numbers,” an owner or manager should be able to run one command and know exactly what is happening in the business.

---

# Product name

## `hcp`

Example:

```bash
hcp brief today
hcp leads speed-to-lead
hcp sales funnel --last 30d
hcp estimates stale --days 7
hcp jobs stalled
hcp tech scorecard
hcp fulfillment bottlenecks
hcp report owner-daily --send slack
```

---

# V1 architecture

## 1. API connector

Use the Housecall Pro API for:

- customers
- leads
- estimates
- jobs
- job appointments
- employees
- invoices
- notes
- tags
- attachments
- schedule windows
- checklists

## 2. Local data mirror

Build a local SQLite/Postgres mirror:

```bash
hcp sync
hcp sync --since yesterday
hcp sync --watch
```

Tables:

- customers
- leads
- jobs
- estimates
- appointments
- employees
- invoices
- job_notes
- estimate_options
- lead_sources
- events
- activity_log
- daily_metrics

## 3. CLI command layer

The API says:

```bash
GET /jobs
GET /estimates
GET /customers
```

The CLI says:

```bash
hcp jobs stalled
hcp estimates unsold --last 30d
hcp revenue leakage
hcp owner brief
```

## 4. MCP/agent mode

Expose the CLI as an MCP server:

```bash
hcp mcp serve
```

---

# V1 command structure

# A. Owner brief commands

```bash
hcp brief today
hcp brief yesterday
hcp brief week
hcp brief month
```

Include:

- new leads
- booked appointments
- estimates created
- estimates sold
- revenue booked
- revenue completed
- unsold estimate value
- stale opportunity value
- jobs completed
- jobs delayed
- invoices unpaid
- top techs
- marketing source performance

---

# B. Leads commands

```bash
hcp leads today
hcp leads yesterday
hcp leads --source google
hcp leads --uncontacted
hcp leads --unbooked
hcp leads stale --minutes 15
hcp leads speed-to-lead
hcp leads by-source --last 30d
```

Metrics:

- lead source
- time to first response
- appointment conversion rate
- stale lead count
- revenue by source
- lead source close rate

---

# C. Funnel commands

```text
Lead → Appointment → Estimate → Sold Job → Completed Job → Paid Invoice
```

Commands:

```bash
hcp funnel --last 30d
hcp funnel --source google
hcp funnel --employee "Sarah"
hcp funnel leakage
hcp funnel compare --this-month --last-month
```

Core metrics:

- leads created
- appointment conversion rate
- estimate conversion rate
- close rate
- sold revenue
- completed revenue
- paid revenue
- average sales cycle
- average ticket

Important output:

```text
Last 30 days:
- 42 leads never became appointments
- 18 appointments never received estimates
- $184,500 in estimates remain unsold
- 9 sold jobs have not been scheduled
- $37,200 in completed jobs remain unpaid
```

---

# D. Sales commands

```bash
hcp estimates open
hcp estimates unsold --last 30d
hcp estimates stale --days 3
hcp estimates high-value --min 5000
hcp estimates no-followup
hcp estimates by-employee
hcp estimates close-rate
hcp sales leaderboard
```

---

# E. Fulfillment commands

```bash
hcp jobs today
hcp jobs tomorrow
hcp jobs active
hcp jobs unscheduled
hcp jobs delayed
hcp jobs stalled
hcp jobs completed-yesterday
hcp jobs missing-invoice
hcp jobs missing-notes
hcp jobs missing-photos
```

Metrics:

- sold-to-scheduled time
- start-to-complete time
- invoice-to-paid time
- jobs completed per tech
- revenue per tech
- jobs missing notes/photos/checklists

---

# F. Technician commands

```bash
hcp techs active
hcp techs today
hcp tech scorecard --last 30d
hcp tech average-ticket
hcp tech revenue
hcp tech leaderboard
```

Scorecard metrics:

- jobs completed
- completed revenue
- average ticket
- callbacks/revisits
- checklist compliance
- customer ratings

---

# G. Dispatch / schedule commands

```bash
hcp schedule today
hcp schedule tomorrow
hcp schedule gaps
hcp schedule overloaded
hcp dispatch risks
```

Example output:

```text
Tomorrow’s dispatch risks:
- 3 jobs unscheduled
- 2 techs over capacity
- 4 jobs missing customer confirmation
- 1 high-value install has no assigned tech
```

---

# H. Marketing commands

```bash
hcp marketing sources --last 30d
hcp marketing roi
hcp marketing booked-rate
hcp marketing close-rate
hcp marketing revenue-by-source
```

Metrics:

- leads by source
- appointments by source
- close rate by source
- revenue by source
- average ticket by source

---

# I. Collections / cash commands

```bash
hcp invoices open
hcp invoices overdue
hcp invoices aging
hcp cash collected --yesterday
hcp ar report
```

---

# J. Automation commands

```bash
hcp report owner-daily --at 7am --send email
hcp report ops-daily --send slack
hcp alert stale-leads --minutes 10
hcp alert high-value-estimate --min 10000
```

---

# The V1 “wow” commands

```bash
hcp brief today
hcp leads stale --minutes 15
hcp funnel --last 30d
hcp funnel leakage
hcp estimates unsold --last 30d
hcp estimates no-followup
hcp jobs stalled
hcp fulfillment bottlenecks
hcp tech scorecard --last 30d
hcp marketing sources --last 30d
```

---

# Recommended V1 build order

## Sprint 1: Foundation

- Auth
- API client
- local DB
- sync engine
- terminal output

Commands:

```bash
hcp auth login
hcp sync
hcp customers list
hcp jobs list
hcp estimates list
hcp leads list
```

## Sprint 2: Operator reports

```bash
hcp brief today
hcp funnel --last 30d
hcp estimates unsold
hcp jobs active
hcp invoices open
```

## Sprint 3: Leakage detection

```bash
hcp leads stale
hcp estimates no-followup
hcp jobs stalled
hcp jobs completed-not-invoiced
hcp funnel leakage
```

## Sprint 4: Automation

- Slack delivery
- email delivery
- CSV export
- scheduled reports

## Sprint 5: MCP mode

```bash
hcp mcp serve
```

---

# Best positioning

Do not call this “an AI automation.”

Call it:

> An open-source command layer for home-service operators running Housecall Pro.

Or:

> A CLI and AI-agent interface for turning Housecall Pro data into daily operating intelligence.

Rozeta positioning:

> We are building the operator-native command layer for home-service CRMs, starting with Housecall Pro and expanding to ServiceTitan.
