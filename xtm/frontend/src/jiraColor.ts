// Parses Jira color macros — {color:VALUE}TEXT{color} — out of a string into a
// flat list of segments. Each segment is plain text or text carrying a color.
// Nested macros resolve innermost-wins for any given character; adjacent and
// repeated macros are supported. An invalid color value keeps the inner text
// but drops the color (and the macro markers).
//
// Examples:
//   splitColorSegments("a {color:#f00}b{color} c")
//     => [{text:"a "}, {text:"b", color:"#f00"}, {text:" c"}]
//   splitColorSegments("{color:#ffbdad}00{color} {color:#57d9a3}00{color}")
//     => [{text:"00",color:"#ffbdad"}, {text:" "}, {text:"00",color:"#57d9a3"}]
//   splitColorSegments("{color:bogus!}x{color}")  => [{text:"x"}]
//   splitColorSegments("plain")                   => [{text:"plain"}]
export interface ColorSegment {
  text: string;
  color?: string;
}

const OPEN = /\{color:([^}]*)\}/;

// A conservative validator: 3/6-digit hex, or a small set of CSS color names
// Jira commonly emits. Anything else is treated as "no color".
const HEX = /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;
const NAMED = new Set([
  "red", "green", "blue", "black", "white", "gray", "grey", "orange",
  "yellow", "purple", "teal", "navy", "maroon", "olive", "silver", "lime",
]);

function validColor(raw: string): string | undefined {
  const v = raw.trim();
  if (HEX.test(v)) return v;
  if (NAMED.has(v.toLowerCase())) return v.toLowerCase();
  return undefined;
}

export function splitColorSegments(input: string): ColorSegment[] {
  const out: ColorSegment[] = [];
  // Stack of currently-open colors; the top is the active color.
  const stack: Array<string | undefined> = [];
  let rest = input;

  const push = (text: string) => {
    if (!text) return;
    const color = stack.length ? stack[stack.length - 1] : undefined;
    // Merge with the previous segment when the color matches, to keep output tidy.
    const prev = out[out.length - 1];
    if (prev && prev.color === color) prev.text += text;
    else out.push(color ? { text, color } : { text });
  };

  while (rest.length > 0) {
    const open = OPEN.exec(rest);
    const closeIdx = rest.indexOf("{color}");

    // Whichever marker comes first (or none) decides the next emit.
    const openIdx = open ? open.index : -1;
    if (openIdx === -1 && closeIdx === -1) {
      push(rest);
      break;
    }
    const nextIsOpen =
      openIdx !== -1 && (closeIdx === -1 || openIdx < closeIdx);

    if (nextIsOpen && open) {
      push(rest.slice(0, openIdx));
      stack.push(validColor(open[1]));
      rest = rest.slice(openIdx + open[0].length);
    } else {
      // a {color} close
      push(rest.slice(0, closeIdx));
      if (stack.length) stack.pop();
      rest = rest.slice(closeIdx + "{color}".length);
    }
  }
  return out;
}

// Minimal hast node shapes (avoids a hard dependency on @types/hast).
interface HastText {
  type: "text";
  value: string;
}
interface HastElement {
  type: "element";
  tagName: string;
  properties?: Record<string, unknown>;
  children: HastNode[];
}
type HastNode = HastText | HastElement | { type: string; children?: HastNode[] };

function colorSpan(seg: ColorSegment): HastNode {
  if (!seg.color) return { type: "text", value: seg.text } as HastText;
  return {
    type: "element",
    tagName: "span",
    // react-markdown parses this style string into a React style object.
    properties: { style: `color:${seg.color}` },
    children: [{ type: "text", value: seg.text } as HastText],
  } as HastElement;
}

// rehype plugin: replace text nodes containing color macros with a mix of text
// and styled <span> nodes. Builds element nodes programmatically (never enables
// raw HTML), so no XSS surface is opened.
export function rehypeJiraColor() {
  return (tree: { children?: HastNode[] }) => {
    const walk = (node: { children?: HastNode[] }) => {
      if (!node.children) return;
      const next: HastNode[] = [];
      for (const child of node.children) {
        if ((child as HastText).type === "text") {
          const value = (child as HastText).value;
          if (value.includes("{color")) {
            for (const seg of splitColorSegments(value)) next.push(colorSpan(seg));
          } else {
            next.push(child);
          }
        } else {
          walk(child as { children?: HastNode[] });
          next.push(child);
        }
      }
      node.children = next;
    };
    walk(tree);
  };
}
