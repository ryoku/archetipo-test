#!/bin/bash
# PostToolUse hook: after editing a web source file, run ESLint + tsc type-check.
# Exits 0 in all cases so Claude is never blocked — output is shown as feedback.

INPUT=$(cat)
FILE=$(echo "$INPUT" | jq -r '.tool_input.file_path // ""')

WEB_ROOT=/home/ryoku/repos/archetipo-test/web

case "$FILE" in
  *.ts|*.tsx)
    # Only act on files inside this project's web directory
    if [[ "$FILE" != "$WEB_ROOT/"* ]]; then
      exit 0
    fi

    cd "$WEB_ROOT" || exit 0

    echo "=== ESLint: $FILE ==="
    npx eslint "$FILE" 2>&1
    LINT_EXIT=$?

    echo "=== TypeScript (tsc --noEmit) ==="
    pnpm exec tsc --noEmit 2>&1
    TSC_EXIT=$?

    if [[ $LINT_EXIT -ne 0 || $TSC_EXIT -ne 0 ]]; then
      echo ""
      echo "web-check: issues found (lint=$LINT_EXIT, tsc=$TSC_EXIT)"
    else
      echo ""
      echo "web-check: OK"
    fi
    ;;
esac

exit 0
