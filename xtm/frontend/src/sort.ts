// Numeric-aware comparison for Jira-style issue keys, mirroring the backend
// keyNumericOrderExpr: the trailing digit run sorts numerically (so "DEMO-9"
// precedes "DEMO-10"); the leading non-numeric part breaks ties lexically.
export function keyCompare(a: string, b: string): number {
  const pa = splitKey(a);
  const pb = splitKey(b);
  if (pa.prefix !== pb.prefix) return pa.prefix < pb.prefix ? -1 : 1;
  if (pa.num !== pb.num) return pa.num - pb.num;
  return a < b ? -1 : a > b ? 1 : 0;
}

function splitKey(k: string): { prefix: string; num: number } {
  const m = /^(.*?)(\d+)\s*$/.exec(k ?? "");
  if (!m) return { prefix: k ?? "", num: -1 };
  return { prefix: m[1], num: parseInt(m[2], 10) };
}

// Case-insensitive string compare; empty strings sort last so blanks don't lead.
export function cmpStr(a: string, b: string): number {
  const x = (a ?? "").toLowerCase();
  const y = (b ?? "").toLowerCase();
  if (!x && y) return 1;
  if (x && !y) return -1;
  return x < y ? -1 : x > y ? 1 : 0;
}

// Flip a comparison result when descending.
export function applyDir(cmp: number, desc: boolean): number {
  return desc ? -cmp : cmp;
}
