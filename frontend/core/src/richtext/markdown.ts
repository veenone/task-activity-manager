import { marked } from "marked";
import type { MarkedToken, Token, Tokens } from "marked";
import type { Block, Inline } from "./ast";

// parseMarkdown turns GFM Markdown into the shared AST by mapping marked's
// own lexer output (marked.lexer, not marked.parse: we never touch its HTML
// renderer, so there is no HTML to sanitize or escape on our side). marked
// already walks the source character by character with its own linear
// tokenizer (D3's concern for wiki.ts's hand-written scanner), so this file
// is purely a shape translation from marked's tokens to ours.
export function parseMarkdown(text: string): Block[] {
  const tokens = marked.lexer(text, { gfm: true }) as MarkedToken[];
  const blocks: Block[] = [];
  for (const token of tokens) {
    const block = mapBlock(token);
    if (block) blocks.push(block);
  }
  return blocks;
}

// Outside-voice fix 5: marked's lexer can return text and codespan token
// text already HTML-escaped; decode exactly these five entities once so
// React never shows them literally. A single-pass regex (rather than five
// sequential replaces) means "&amp;lt;" decodes to the literal text "&lt;"
// and not a second time to "<".
const ENTITY_RE = /&amp;|&lt;|&gt;|&quot;|&#39;/g;
const ENTITIES: Record<string, string> = {
  "&amp;": "&",
  "&lt;": "<",
  "&gt;": ">",
  "&quot;": '"',
  "&#39;": "'",
};

function decode(text: string): string {
  return text.replace(ENTITY_RE, (m) => ENTITIES[m]);
}

function mapBlock(token: MarkedToken): Block | null {
  switch (token.type) {
    case "heading":
      // GFM only ever produces depth 1-6 (a 7th '#' is not a heading), so
      // this cast mirrors what the grammar already guarantees.
      return { t: "h", level: token.depth as 1 | 2 | 3 | 4 | 5 | 6, children: mapInlines(token.tokens) };
    case "paragraph":
      return { t: "p", children: mapInlines(token.tokens) };
    case "list":
      return { t: "list", ordered: token.ordered, items: token.items.map(mapListItem) };
    case "table":
      return {
        t: "table",
        rows: [
          token.header.map((cell) => ({ header: true, children: mapInlines(cell.tokens) })),
          ...token.rows.map((row) => row.map((cell) => ({ header: false, children: mapInlines(cell.tokens) }))),
        ],
      };
    case "code":
      return { t: "codeblock", lang: token.lang ?? "", text: token.text };
    case "blockquote":
      return { t: "quote", children: mapBlocks(token.tokens) };
    case "hr":
      return { t: "rule" };
    case "space":
      return null;
    default:
      // Constraint: raw HTML in Markdown renders as literal text, never as
      // markup. A block-level HTML token (or anything else this AST has no
      // shape for) falls back to its own literal source rather than being
      // dropped, so user content is never silently lost.
      return token.raw.trim() ? { t: "p", children: [{ t: "text", text: decode(token.raw) }] } : null;
  }
}

function mapBlocks(tokens: Token[]): Block[] {
  const blocks: Block[] = [];
  for (const token of tokens) {
    const block = mapBlock(token as MarkedToken);
    if (block) blocks.push(block);
  }
  return blocks;
}

function mapListItem(item: Tokens.ListItem): { checked?: boolean; children: Block[] } {
  const children: Block[] = [];
  for (const token of item.tokens as MarkedToken[]) {
    if (token.type === "checkbox") continue; // item.checked already carries this
    if (token.type === "text") {
      // A tight list item's own content arrives as a "text" token wrapping
      // inline tokens rather than as a "paragraph".
      children.push({ t: "p", children: mapInlines((token as Tokens.Text).tokens ?? []) });
      continue;
    }
    const block = mapBlock(token);
    if (block) children.push(block);
  }
  return item.task ? { checked: !!item.checked, children } : { children };
}

function mapInlines(tokens: Token[]): Inline[] {
  const nodes: Inline[] = [];
  for (const token of tokens) push(nodes, mapInline(token as MarkedToken));
  return nodes;
}

// push merges a run of adjacent text nodes into one, the same shape wiki.ts's
// pushText keeps: an <html> tag straddled by plain text (e.g. "<b>hi</b>")
// reads back as a single literal text node rather than three fragments.
function push(nodes: Inline[], mapped: Inline | null): void {
  if (mapped === null) return;
  const last = nodes[nodes.length - 1];
  if (mapped.t === "text" && last?.t === "text") {
    last.text += mapped.text;
    return;
  }
  nodes.push(mapped);
}

function mapInline(token: MarkedToken): Inline | null {
  switch (token.type) {
    case "text":
    case "escape":
      return { t: "text", text: decode(token.text) };
    case "strong":
      return { t: "mark", mark: "bold", children: mapInlines(token.tokens) };
    case "em":
      return { t: "mark", mark: "italic", children: mapInlines(token.tokens) };
    case "del":
      return { t: "mark", mark: "strike", children: mapInlines(token.tokens) };
    case "codespan":
      return { t: "code", text: decode(token.text) };
    case "link":
      return { t: "link", href: token.href, children: mapInlines(token.tokens) };
    case "image":
      return { t: "image", name: imageName(token.href) };
    case "br":
      return { t: "br" };
    case "html":
      // Constraint: raw HTML renders as literal text, never interpreted.
      return { t: "text", text: decode(token.text) };
    case "space":
      return null;
    default:
      return "raw" in token && token.raw ? { t: "text", text: decode(token.raw) } : null;
  }
}

function imageName(href: string): string {
  const path = href.split(/[?#]/)[0];
  const parts = path.split("/");
  return parts[parts.length - 1] || href;
}
