// The rich text syntax tree both parsers (wiki.ts, markdown.ts) build and
// the renderer walks. Kept intentionally small: one node per thing the
// renderer draws differently, nothing that only one format happens to have.

export type RichFormat = "wiki" | "markdown";

export type Mark = "bold" | "italic" | "underline" | "strike" | "sup" | "sub";

export type Inline =
  | { t: "text"; text: string }
  | { t: "mark"; mark: Mark; children: Inline[] }
  | { t: "code"; text: string }
  | { t: "link"; href: string; children: Inline[] }
  | { t: "br" }
  | { t: "image"; name: string };

export type Block =
  | { t: "p"; children: Inline[] }
  | { t: "h"; level: 1 | 2 | 3 | 4 | 5 | 6; children: Inline[] }
  | { t: "list"; ordered: boolean; items: { checked?: boolean; children: Block[] }[] }
  | { t: "table"; rows: { header: boolean; children: Inline[] }[][] }
  | { t: "codeblock"; lang: string; text: string }
  | { t: "quote"; children: Block[] }
  | { t: "rule" }
  | { t: "panel"; title: string; children: Block[] };
