#!/bin/bash
# PostToolUse hook: after editing a Go source file, run golangci-lint + go build.
# Exits 0 in all cases so Claude is never blocked — output is shown as feedback.

INPUT=$(cat)
FILE=$(echo "$INPUT" | jq -r '.tool_input.file_path // ""')

GO_ROOT=/home/ryoku/repos/archetipo-test

case "$FILE" in
  *.go)
    if [[ "$FILE" != "$GO_ROOT/"* ]]; then
      exit 0
    fi

    cd "$GO_ROOT" || exit 0

    echo "=== golangci-lint: $FILE ==="
    golangci-lint run "$FILE" 2>&1
    LINT_EXIT=$?

    echo "=== go build ./... ==="
    go build ./... 2>&1
    BUILD_EXIT=$?

    if [[ $LINT_EXIT -ne 0 || $BUILD_EXIT -ne 0 ]]; then
      echo ""
      echo "go-check: issues found (lint=$LINT_EXIT, build=$BUILD_EXIT)"
    else
      echo ""
      echo "go-check: OK"
    fi
    ;;
esac

exit 0
