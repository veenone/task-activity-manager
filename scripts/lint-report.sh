#!/bin/sh
# Run eslint once and write the JSON the ratchet counts from.
# eslint exits non-zero while a backlog exists, which is expected: the ratchet
# holds the counts, the linter itself does not block.
set -u
npx eslint frontend/core/src tam/frontend/src xtm/frontend/src \
  -f json -o eslint-report.json || true
[ -s eslint-report.json ] || { echo "lint-report: eslint wrote no report"; exit 2; }
echo "lint-report: $(node -e "console.log(require('./eslint-report.json').reduce((n,f)=>n+f.messages.length,0))") problems"
