# Regrada

**CI gate for LLM behavior** — record real model traffic, turn it into test cases, and block regressions in CI.

Regrada:
- Records LLM API calls via an HTTP proxy (`regrada record`)
- Converts recorded traces into portable YAML cases + baseline snapshots (`regrada accept`)
- Runs cases repeatedly, diffs vs baselines, and enforces configurable policies (`regrada test`)
- Produces CI-friendly reports (stdout summary, Markdown, JUnit) and a GitHub Action

## Installation

### macOS / Linux

```sh
curl -fsSL https://regrada.com/install.sh | sh
```

```sh
wget -qO- https://regrada.com/install.sh | sh
```

The installer downloads a prebuilt binary and installs it to `~/.local/bin/regrada`.
If `regrada` isn't found, add `~/.local/bin` to your `PATH` (the installer prints the exact snippet to copy/paste).

### Windows

The installer targets macOS/Linux. On Windows, run Regrada via WSL:

```sh
curl -fsSL https://regrada.com/install.sh | sh
```

### Build from source

```sh
mkdir -p bin
go build -o ./bin/regrada .
./bin/regrada version
```

Or:

```sh
make build
./bin/regrada version
```

## Quick Start (Local)

1) Initialize config + an example case:

```sh
regrada init
```

2) Configure a provider (OpenAI is implemented; others are scaffolded but not runnable yet):

```sh
export OPENAI_API_KEY="..."
```

Edit `regrada.yml`:

```yaml
providers:
  default: openai
  openai:
    model: gpt-4o-mini
```

3) Start with local baselines:

```yaml
baseline:
  mode: local
```

4) Generate baselines and run tests:

```sh
regrada baseline
regrada test
```

## Core Concepts

### Cases

A **case** is a YAML file (default: `regrada/cases/**/*.yml`) containing a prompt (chat messages or structured input) plus optional assertions.

### Assertions vs Policies (important)

- **Case assertions** (`assert:` in a case file) mark individual runs as pass/fail and feed metrics like `pass_rate`.
- **Policies** (`policies:` in `regrada.yml`) decide what counts as a *warning* or *error* in CI.

If you want failing assertions to break CI, add an `assertions` policy (example below).

### Baselines

A **baseline** is a stored snapshot (golden output + aggregate metrics) used for regression checks.

Regrada stores baselines under the snapshot directory (default: `.regrada/snapshots/`), keyed by:

- Case ID
- Provider + model
- Sampling params (temperature/top_p/max tokens/stop)
- System prompt content

Changing any of the above produces a different baseline key and requires a new baseline file.

## CLI

### `regrada init`

Creates `regrada.yml`, an example case, and runtime directories:

```sh
regrada init
```

Flags:

- `--path regrada.yml` (default: `regrada.yml`)
- `--force` overwrite existing config
- `--non-interactive` use defaults

### `regrada record`

Starts an HTTP proxy to capture LLM traffic (default: forward proxy with HTTPS MITM):

```sh
regrada record
regrada record -- python app.py
regrada record -- npm test
```

Recorded traces are written to `.regrada/traces/` (JSONL) and sessions to `.regrada/sessions/`.

### `regrada accept`

Converts traces from the latest (or specified) session into cases and baselines:

```sh
regrada accept
regrada accept --session .regrada/sessions/20250101-120000.json
```

Flags:

- `--config, -c`: config file path (default: `regrada.yml`/`regrada.yaml`)
- `--session`: session file path (default: latest)

### `regrada baseline`

Runs all discovered cases once and writes baseline snapshots:

```sh
regrada baseline
```

Flags:

- `--config, -c`: config file path (default: `regrada.yml`/`regrada.yaml`)

### `regrada test`

Runs cases, diffs against baselines, evaluates policies, and writes reports:

```sh
regrada test
```

Flags:

- `--config, -c`: config file path (default: `regrada.yml`/`regrada.yaml`)

### `regrada ca`

Manages the local Root CA required for forward-proxy HTTPS interception:

```sh
regrada ca init
regrada ca install
regrada ca status
regrada ca uninstall
```

Flags:

- `--config, -c`: config file path (default: `regrada.yml`/`regrada.yaml`)

## Configuration (`regrada.yml`)

Minimal working config (OpenAI):

```yaml
version: 1

providers:
  default: openai
  openai:
    model: gpt-4o-mini

baseline:
  mode: local

policies:
  - id: assertions
    severity: error
    check:
      type: assertions
      min_pass_rate: 1.0
```

### Providers

Implemented today:

- `openai` (Chat Completions)
- `mock` (useful for wiring/tests; returns `"mock response"`)

Scaffolded in config but not implemented in the runner yet:

- `anthropic`, `azure_openai`, `bedrock`

OpenAI credential precedence:

1. `providers.openai.api_key_env` (default: `OPENAI_API_KEY`)
2. `providers.openai.api_key`
3. `OPENAI_API_KEY`

Model precedence:

1. `providers.openai.model`
2. `OPENAI_MODEL`

### Case discovery

Defaults (can be overridden under `cases:`):

- roots: `["regrada/cases"]`
- include globs: `["**/*.yml", "**/*.yaml"]`
- exclude globs: `["**/README.*"]`

### Baselines

Baseline modes:

- `baseline.mode: local` reads snapshots from the local filesystem.
- `baseline.mode: git` reads snapshots from a git ref via `git show <ref>:<path>`.

Example `git` baseline config:

```yaml
baseline:
  mode: git
  git:
    ref: origin/main
    snapshot_dir: .regrada/snapshots
```

Note: to use `git` mode in CI, your checkout must fetch the baseline ref (see GitHub Action section).

### Reports

By default, `regrada test` writes:

- A one-line summary to stdout: `Total: N | Passed: N | Warned: N | Failed: N`
- A Markdown report to `.regrada/report.md`

Enable JUnit output:

```yaml
report:
  format: [summary, markdown, junit]
  junit:
    path: .regrada/junit.xml
```

### CI behavior

By default, Regrada fails the run when any policy violation with `severity: error` is present.

To also fail on warnings:

```yaml
ci:
  fail_on:
    - severity: error
    - severity: warn
```

### Capture / Record settings

Forward proxy (default) uses HTTPS MITM and requires `regrada ca init` + trust setup.

```yaml
capture:
  enabled: true
  mode: proxy # proxy | off
  proxy:
    listen: 127.0.0.1:8080
    mode: forward # forward | reverse
    ca_path: .regrada/ca
    allow_hosts:
      - api.openai.com
  redact:
    enabled: true
    presets: [pii_basic, secrets]
```

## Case Format (`regrada/cases/**/*.yml`)

Example:

```yaml
id: greeting.hello
tags: [smoke]

request:
  messages:
    - role: system
      content: You are a concise assistant.
    - role: user
      content: Say hello and ask for a name.
  params:
    temperature: 0.2
    top_p: 1.0

assert:
  text:
    contains: ["hello"]
    max_chars: 120
```

Notes:

- `request` must specify **either** `messages` **or** `input` (a YAML map).
- Roles must be `system`, `user`, `assistant`, or `tool`.
- `assert.json.schema` and `assert.json.path` are parsed/validated but **not enforced yet** by the runner.

## Policies (`regrada.yml`)

Policies are how you turn runs/diffs into CI gates.

Common setup:

```yaml
policies:
  - id: assertions
    severity: error
    check:
      type: assertions
      min_pass_rate: 1.0

  - id: no_pii
    severity: error
    check:
      type: pii_leak
      detector: pii_strict
      max_incidents: 0

  - id: stable_text
    severity: warn
    check:
      type: variance
      metric: token_jaccard
      max_p95: 0.35 # variance = 1 - token_jaccard (lower is stricter)
```

Supported `check.type` values:

- `assertions` (required: `min_pass_rate`)
- `json_valid` (optional: `min_pass_rate`, defaults to `1.0`)
- `text_contains` (required: `phrases`, optional: `min_pass_rate`)
- `text_not_contains` (required: `phrases`, optional: `max_incidents`, defaults to `0`)
- `pii_leak` (required: `detector`, optional: `max_incidents`)
- `variance` (required by config validation: `metric`; required by engine: `max_p95`)
- `refusal_rate` (required: `max` and/or `max_delta`)
- `latency` (required: `p95_ms` and at least one of `p95_ms.max` or `p95_ms.max_delta`)
- `json_schema` (validated but **not implemented yet**)

## Recording workflow

### Forward proxy (recommended)

1) Generate and trust the local CA:

```sh
regrada ca init
regrada ca install
```

2) Run your app/tests through the proxy:

```sh
regrada record -- ./run-my-tests.sh
```

3) Convert the latest session into cases + baselines:

```sh
regrada accept
```

### Reverse proxy (no MITM)

Set `capture.proxy.mode: reverse` and point your LLM base URL at the proxy. This mode does not require installing the CA, but your application must be configurable to talk to the proxy instead of the upstream API.

## Baselines in Git (recommended for CI)

1) Ensure your snapshot directory is version-controlled.

By default, `regrada init` adds `.regrada/` to `.gitignore`. If you keep snapshots under `.regrada/snapshots/`, un-ignore that directory (recommended) rather than removing the ignore entirely.

Example `.gitignore` snippet:

```gitignore
.regrada/*
!.regrada/snapshots/
!.regrada/snapshots/**
```

2) On your baseline branch (e.g. `main`), generate and commit snapshots:

```sh
regrada baseline
git add .regrada/snapshots regrada/cases regrada.yml
git commit -m "Update Regrada baselines"
```

3) In PR branches/CI, run `regrada test` with `baseline.mode: git` and `baseline.git.ref: origin/main`.

## GitHub Action

```yaml
name: Regrada
on:
  pull_request:

jobs:
  regrada:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0 # required for baseline.mode=git with origin/main

      - uses: regrada-ai/regrada@v1
        with:
          config: regrada.yml
          comment-on-pr: true
          working-directory: .
```

### Action inputs

| Input | Description | Default |
| --- | --- | --- |
| `config` | Path to `regrada.yml`/`regrada.yaml` | `regrada.yml` |
| `comment-on-pr` | Post `.regrada/report.md` as a PR comment | `true` |
| `working-directory` | Directory to run `regrada test` in | `.` |

### Action outputs

| Output | Description |
| --- | --- |
| `total` | Total number of cases |
| `passed` | Number of passed cases |
| `warned` | Number of warned cases |
| `failed` | Number of failed cases |
| `result` | `success`, `warning`, or `failure` |

## Exit Codes

`regrada test` uses exit codes to help CI distinguish failure modes:

- `0`: no failing policy violations
- `1`: internal error (provider/report/etc.)
- `2`: policy violations (as configured by `ci.fail_on`)
- `3`: invalid config / no cases discovered
- `4`: missing baseline snapshot
- `5`: evaluation error (provider call failed, timeout, etc.)

## Troubleshooting

- **“config not found”**: create `regrada.yml` (`regrada init`) or pass `--config`.
- **Exit code 4 / baseline missing**: run `regrada baseline` on your baseline ref and commit snapshots; ensure CI fetches `baseline.git.ref`.
- **OpenAI auth errors**: set `OPENAI_API_KEY` (or `providers.openai.api_key[_env]`).
- **Recording HTTPS fails**: run `regrada ca init` + `regrada ca install`, and confirm `capture.proxy.allow_hosts` includes your provider host.
