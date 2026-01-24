#!/bin/bash
set -e

CONFIG_PATH="$1"

if [ -z "$CONFIG_PATH" ]; then
  CONFIG_PATH="regrada.yml"
fi

SUMMARY_FILE=".regrada/summary.txt"
mkdir -p .regrada

set +e
regrada test --config "$CONFIG_PATH" | tee "$SUMMARY_FILE"
EXIT_CODE=$?
set -e

SUMMARY_LINE=$(grep -E "^Total:" "$SUMMARY_FILE" | tail -n 1)
TOTAL=$(echo "$SUMMARY_LINE" | sed -E 's/.*Total: ([0-9]+).*/\1/')
PASSED=$(echo "$SUMMARY_LINE" | sed -E 's/.*Passed: ([0-9]+).*/\1/')
WARNED=$(echo "$SUMMARY_LINE" | sed -E 's/.*Warned: ([0-9]+).*/\1/')
FAILED=$(echo "$SUMMARY_LINE" | sed -E 's/.*Failed: ([0-9]+).*/\1/')

if [ -z "$TOTAL" ] || [ "$TOTAL" = "$SUMMARY_LINE" ]; then
  TOTAL=0
  PASSED=0
  WARNED=0
  FAILED=0
fi

echo "total=$TOTAL" >> "$GITHUB_OUTPUT"
echo "passed=$PASSED" >> "$GITHUB_OUTPUT"
echo "warned=$WARNED" >> "$GITHUB_OUTPUT"
echo "failed=$FAILED" >> "$GITHUB_OUTPUT"

echo "result=success" >> "$GITHUB_OUTPUT"
if [ "$FAILED" -gt 0 ]; then
  echo "result=failure" >> "$GITHUB_OUTPUT"
elif [ "$WARNED" -gt 0 ]; then
  echo "result=warning" >> "$GITHUB_OUTPUT"
fi

echo "exit_code=$EXIT_CODE" >> "$GITHUB_OUTPUT"
