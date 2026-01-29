# Regrada

**CI for AI systems** — Detect behavioral regressions in LLM-powered apps before they hit production.

Regrada is a testing and continuous integration tool designed specifically for AI/LLM applications. It captures LLM interactions, runs evaluations against test cases, and detects when your AI's behavior changes between commits.

## Installation

### From Source

```bash
curl -fsSL https://regrada.com/install.sh | sh
```

## Quick Start

```bash
# Initialize a new project
regrada init

# Run evaluations
regrada run

# Run in CI mode (exits 1 on regression)
regrada run --ci
```

## GitHub Action

Use Regrada as a GitHub Action to automatically test your AI on every PR:

```yaml
name: AI Tests
on:
  pull_request:
    branches: [main]

jobs:
  regrada:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: regrada-ai/regrada@v1
        with:
          tests: evals/tests.yaml
          baseline: .regrada/baseline.json
          fail-on-regression: true
          comment-on-pr: true
```

### Action Inputs

| Input                | Description                  | Default                  |
| -------------------- | ---------------------------- | ------------------------ |
| `tests`              | Path to test suite file      | `evals/tests.yaml`       |
| `baseline`           | Path to baseline file        | `.regrada/baseline.json` |
| `fail-on-regression` | Fail if regressions detected | `true`                   |
| `fail-on-failure`    | Fail if any test fails       | `false`                  |
| `comment-on-pr`      | Post results as PR comment   | `true`                   |
| `working-directory`  | Working directory for tests  | `.`                      |

### Action Outputs

| Output        | Description                                           |
| ------------- | ----------------------------------------------------- |
| `total`       | Total number of tests                                 |
| `passed`      | Number of passed tests                                |
| `failed`      | Number of failed tests                                |
| `regressions` | Number of regressions                                 |
| `result`      | Overall result: `success`, `failure`, or `regression` |

## Commands

### `regrada init`

Initialize a new Regrada project with interactive prompts:

```bash
regrada init [path]
```

Creates:

- `.regrada.yaml` - Configuration file
- `evals/tests.yaml` - Test suite
- `evals/prompts/` - Prompt templates
- `.regrada/baseline.json` - Initial baseline

### `regrada run`

Run evaluations against your test suite:

```bash
regrada run [flags]
```

**Flags:**

- `-t, --tests` - Path to test suite (default: `evals/tests.yaml`)
- `-b, --baseline` - Path to baseline (default: `.regrada/baseline.json`)
- `-c, --config` - Path to config (default: `.regrada.yaml`)
- `-o, --output` - Output format: `text`, `json`, `github`
- `--ci` - CI mode: exit 1 on regression

### `regrada trace`

Capture LLM calls from your application:

```bash
regrada trace -- your-command [args]
```

Automatically detects and captures calls to:

- OpenAI
- Anthropic
- Azure OpenAI
- Google AI (Gemini)
- Cohere
- Custom endpoints

**Flags:**

- `-o, --output` - Output file (default: `.regrada/traces.json`)
- `-f, --format` - Output format: `json`, `yaml`

## Configuration

`.regrada.yaml`:

```yaml
provider:
  type: openai # openai, anthropic, azure, google, cohere, custom
  model: gpt-4
  api_key_env: OPENAI_API_KEY

capture:
  inputs: true # Capture prompts
  outputs: true # Capture responses
  tool_calls: true # Capture tool/function calls
  metadata: true # Capture tokens, latency, etc.

evals:
  path: evals # Directory for test files
  parallel: 4 # Concurrent test execution

gate:
  max_regressions: 0 # Block PR if exceeded
  min_pass_rate: 0.95 # Minimum pass rate (0-1)

output:
  format: text # text, json, github
  verbose: false
```

## Writing Tests

Tests are defined in YAML with prompts and checks:

```yaml
name: Customer Support Agent Tests
description: Evaluate support agent responses

tests:
  - name: refund_request
    prompt: |
      Customer: I want a refund for order #12345
    checks:
      - schema_valid
      - "tool_called:refund.lookup"
      - "sentiment:empathetic"
      - no_hallucination

  - name: product_question
    prompt: prompts/product_question.txt
    checks:
      - grounded_in_retrieval
      - stays_on_topic
      - "length:<500"
```

### Available Checks

| Check                   | Description                      |
| ----------------------- | -------------------------------- |
| `schema_valid`          | Response matches expected schema |
| `tool_called:name`      | Specific tool was invoked        |
| `no_tool_called`        | No tools were called             |
| `grounded_in_retrieval` | Response uses retrieved context  |
| `no_hallucination`      | No fabricated information        |
| `stays_on_topic`        | Response is relevant to prompt   |
| `sentiment:type`        | Response has expected sentiment  |
| `tone:type`             | Response has expected tone       |
| `length:<N`             | Response under N characters      |
| `response_time:<Nms`    | Response within time limit       |

## Baselines

Baselines capture your AI's expected behavior. Regrada compares current results against the baseline to detect regressions.

```bash
# Create initial baseline
regrada run --output json > .regrada/baseline.json

# Update baseline after intentional changes
regrada run --output json > .regrada/baseline.json
git add .regrada/baseline.json
git commit -m "Update AI baseline"
```

## CI Integration

### GitHub Actions

The recommended approach. See [GitHub Action](#github-action) above.

### Other CI Systems

```bash
# Run with CI mode
regrada run --ci --output json

# Check exit code
# 0 = all tests pass, no regressions
# 1 = failures or regressions detected
```

## Project Structure

```
your-project/
├── .regrada.yaml           # Configuration
├── .regrada/
│   ├── baseline.json       # Baseline results
│   └── results.json        # Latest results
└── evals/
    ├── tests.yaml          # Test definitions
    └── prompts/            # Prompt templates
        ├── refund.txt
        └── greeting.txt
```

## License

This project is proprietary software licensed under the Regrada
proprietary license. All rights reserved.

No use, reproduction, modification, or distribution is permitted
without explicit authorization.
