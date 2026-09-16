// Count eslint problems matching a rule prefix, from the report lint-report.sh
// wrote. Exits 2 when the report is missing: a counter that silently prints 0
// is a gate that scans nothing (AGENTS.md M2).
const fs = require('node:fs');
if (!fs.existsSync('eslint-report.json')) {
  console.error('lint-count: eslint-report.json missing; run sh scripts/lint-report.sh');
  process.exit(2);
}
const prefix = process.argv[2] || '';
const report = JSON.parse(fs.readFileSync('eslint-report.json', 'utf8'));
let n = 0;
for (const file of report) {
  for (const m of file.messages) {
    if ((m.ruleId || '').startsWith(prefix)) n += 1;
  }
}
console.log(n);
