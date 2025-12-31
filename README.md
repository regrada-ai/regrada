# Regrada CLI

A powerful command-line tool for managing regrada workflows.

## Installation

### From Source

```bash
# Clone the repository
git clone <repository-url>
cd regrada

# Download dependencies
make deps

# Build the binary
make build

# Install to GOPATH (optional)
make install
```

## Usage

The Regrada CLI provides four main commands:

### 1. Init - Initialize a Project

```bash
# Initialize in current directory
regrada init

# Initialize in specific directory
regrada init ./my-project

# Force initialization
regrada init --force

# Use custom config
regrada init --config ./custom-config.yml
```

**Flags:**
- `-f, --force`: Force initialization even if project exists
- `-c, --config`: Specify a custom config file

### 2. Trace - Trace Execution

```bash
# Basic trace
regrada trace

# Trace specific target
regrada trace my-function

# Trace with filter
regrada trace --filter "error"

# Trace with custom depth
regrada trace --depth 20

# Output as JSON
regrada trace --output json
```

**Flags:**
- `-f, --filter`: Filter traces by pattern
- `-d, --depth`: Maximum trace depth (default: 10)
- `-o, --output`: Output format (text, json, csv)

### 3. Diff - Compare and Show Differences

```bash
# Compare two files
regrada diff file1.txt file2.txt

# Compare with custom context
regrada diff file1.txt file2.txt --context 5

# Use unified format
regrada diff file1.txt file2.txt --unified

# Ignore whitespace changes
regrada diff file1.txt file2.txt --ignore-whitespace
```

**Flags:**
- `-c, --context`: Number of context lines (default: 3)
- `-u, --unified`: Use unified diff format
- `-w, --ignore-whitespace`: Ignore whitespace changes

### 4. Gate - Manage Gates

```bash
# Show gate status
regrada gate status

# List all gates
regrada gate list

# Enable a gate
regrada gate enable --name my-feature

# Disable a gate
regrada gate disable --name my-feature

# Apply to all gates
regrada gate enable --all
```

**Flags:**
- `-n, --name`: Gate name
- `-a, --all`: Apply to all gates

## Global Flags

All commands support the following global flag:
- `-v, --verbose`: Enable verbose output

## Development

### Build

```bash
make build
```

### Run Tests

```bash
make test
```

### Format Code

```bash
make fmt
```

### Run Linters

```bash
make lint
```

### Clean Build Artifacts

```bash
make clean
```

### Run All Tasks

```bash
make all
```

## Project Structure

```
regrada/
├── cmd/              # Command implementations
│   ├── root.go      # Root command and CLI setup
│   ├── init.go      # Init command
│   ├── trace.go     # Trace command
│   ├── diff.go      # Diff command
│   └── gate.go      # Gate command
├── main.go          # Entry point
├── go.mod           # Go module file
├── Makefile         # Build automation
└── README.md        # This file
```

## License

[Your License Here]
