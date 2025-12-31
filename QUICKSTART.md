# Quick Start Guide

## Prerequisites

- Go 1.21 or later installed
- Git (optional)

## Quick Setup

1. **Download dependencies:**

   ```bash
   go mod download
   ```

2. **Build the CLI:**

   ```bash
   make build
   ```

   or

   ```bash
   go build -o bin/regrada .
   ```

3. **Run the CLI:**
   ```bash
   ./bin/regrada
   ```

## Try the Commands

### Initialize a project

```bash
./bin/regrada init
```

### Run a trace

```bash
./bin/regrada trace --depth 5
```

### Compare files

```bash
./bin/regrada diff file1.txt file2.txt
```

### Manage gates

```bash
./bin/regrada gate list
```

## Install System-Wide

To install the CLI to your GOPATH so you can use it anywhere:

```bash
make install
```

Then you can run:

```bash
regrada --help
```

## Next Steps

1. Implement the actual logic for each command in the respective files under `cmd/`
2. Add tests for each command
3. Add configuration file support
4. Add more flags and options as needed
5. Integrate with your SDK in the `sdk/` directory

## Project Structure

- `main.go` - Entry point
- `cmd/root.go` - Root command setup with Cobra
- `cmd/init.go` - Init command implementation
- `cmd/trace.go` - Trace command implementation
- `cmd/diff.go` - Diff command implementation
- `cmd/gate.go` - Gate command implementation
- `Makefile` - Build automation

Happy coding! 🚀
