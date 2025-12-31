#!/bin/bash
set +e

regrada run \
  --ci \
  --tests "$1" \
  --baseline "$2" \
  --output json > .regrada/results.json 2>&1

EXIT_CODE=$?

# Parse results
if [ -f .regrada/results.json ]; then
  TOTAL=$(jq -r '.total_tests // 0' .regrada/results.json)
  PASSED=$(jq -r '.passed // 0' .regrada/results.json)
  FAILED=$(jq -r '.failed // 0' .regrada/results.json)
  REGRESSIONS=$(jq -r '.regressions // 0' .regrada/results.json)
else
  TOTAL=0
  PASSED=0
  FAILED=0
  REGRESSIONS=0
fi

echo "total=$TOTAL" >> $GITHUB_OUTPUT
echo "passed=$PASSED" >> $GITHUB_OUTPUT
echo "failed=$FAILED" >> $GITHUB_OUTPUT
echo "regressions=$REGRESSIONS" >> $GITHUB_OUTPUT

# Determine result
if [ "$REGRESSIONS" -gt 0 ]; then
  echo "result=regression" >> $GITHUB_OUTPUT
elif [ "$FAILED" -gt 0 ]; then
  echo "result=failure" >> $GITHUB_OUTPUT
else
  echo "result=success" >> $GITHUB_OUTPUT
fi

# Print summary to logs
echo "## Regrada Results" >> $GITHUB_STEP_SUMMARY
echo "" >> $GITHUB_STEP_SUMMARY
echo "| Metric | Value |" >> $GITHUB_STEP_SUMMARY
echo "|--------|-------|" >> $GITHUB_STEP_SUMMARY
echo "| Total | $TOTAL |" >> $GITHUB_STEP_SUMMARY
echo "| Passed | $PASSED |" >> $GITHUB_STEP_SUMMARY
echo "| Failed | $FAILED |" >> $GITHUB_STEP_SUMMARY
echo "| Regressions | $REGRESSIONS |" >> $GITHUB_STEP_SUMMARY

# Determine exit code based on inputs
if [ "$3" = "true" ] && [ "$REGRESSIONS" -gt 0 ]; then
  echo "exit_code=1" >> $GITHUB_OUTPUT
elif [ "$4" = "true" ] && [ "$FAILED" -gt 0 ]; then
  echo "exit_code=1" >> $GITHUB_OUTPUT
else
  echo "exit_code=0" >> $GITHUB_OUTPUT
fi
