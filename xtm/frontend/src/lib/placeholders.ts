// Placeholder references in a parameterized test's text.
//
// Xray substitutes ${NAME} in step text and unstructured definitions with the
// matching dataset parameter, once per iteration. A reference naming no
// parameter never resolves, and Xray's own web UI shows it in red, so marking
// it here follows Xray rather than inventing a rule.
//
// One helper rather than a regex in each of StepRow, the Cucumber editor and
// the generic editor: three copies drift, and the first fix would land in only
// one of them (#267).
//
// Gherkin's <NAME> form is not handled yet. It belongs here when it arrives.

// PLACEHOLDER matches a ${...} reference. The body is lazy and excludes the
// closing brace, so an unclosed "${" never swallows the rest of the text.
const PLACEHOLDER = /\$\{([^{}]*)\}/g;

export interface PlaceholderSegment {
  // The literal text of this segment, so joining every segment's text
  // reproduces the input exactly.
  text: string;
  // The parameter named by this segment, when it is a ${...} reference.
  // Absent on plain text.
  parameter?: string;
  // True when the reference names no parameter in the dataset. Only set
  // alongside parameter.
  unknown?: boolean;
}

// splitPlaceholders breaks text into plain and ${...} segments, marking each
// reference as known or unknown against the dataset's parameter names.
//
// Matching is case sensitive, as Xray's substitution is. Whitespace inside the
// braces is ignored, so "${ NAME }" resolves, but the segment keeps the
// original text so the rendered output is unchanged.
export function splitPlaceholders(text: string, known: string[]): PlaceholderSegment[] {
  if (!text) return [];

  const names = new Set(known);
  const out: PlaceholderSegment[] = [];
  let last = 0;

  PLACEHOLDER.lastIndex = 0;
  for (let m = PLACEHOLDER.exec(text); m !== null; m = PLACEHOLDER.exec(text)) {
    if (m.index > last) {
      out.push({ text: text.slice(last, m.index) });
    }
    const name = m[1].trim();
    out.push({ text: m[0], parameter: name, unknown: !names.has(name) });
    last = m.index + m[0].length;
  }
  if (last < text.length) {
    out.push({ text: text.slice(last) });
  }
  return out;
}
