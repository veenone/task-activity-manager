import { Fragment, useMemo } from "react";
import type { ReactElement, ReactNode } from "react";
import type { Block, Inline, RichFormat } from "./ast";
import { parseRich } from "./detect";
import { isAllowedLink } from "../lib/links";

// D3/D6 size guard: cut before parsing, not after. 200 KB of a description
// nobody will read past on screen is wasted parse time either way, and
// Jira's own web view is the place to read the rest.
export const RICH_TEXT_LIMIT = 200_000;

const SIZE_SENTENCE = "Showing the first 200 KB. Open in Jira for the rest.";

const MARK_TAGS = {
  bold: "strong",
  italic: "em",
  underline: "u",
  strike: "s",
  sup: "sup",
  sub: "sub",
} as const;

const HEADING_TAGS = {
  1: "h1",
  2: "h2",
  3: "h3",
  4: "h4",
  5: "h5",
  6: "h6",
} as const;

interface Props {
  text: string;
  format?: RichFormat | "auto";
  // inline drops every block but paragraphs and headings, joins what is
  // left with a space, and renders marks as plain text (D5): the summary
  // field is one line Jira never wiki- or Markdown-renders, so only its
  // inline code and links get to look different from what was typed.
  inline?: boolean;
  // projectKey, together with onIssueKey, turns a bare "PLAT-409" in plain
  // text into a button; a key belonging to another project always stays
  // text, and no split happens at all without onIssueKey.
  projectKey?: string;
  onOpenLink: (url: string) => void;
  onIssueKey?: (key: string) => void;
}

interface RenderOpts {
  // issueKeyPattern is built once per render, from projectKey alone (see
  // RichText's own useMemo), and reused across every text leaf: a fresh
  // RegExp per leaf was measurable render cost in the one component whose
  // whole point is being cheap to re-render. It carries the "g" flag, so
  // renderText resets lastIndex before each leaf's scan; a global regex
  // that runs to exhaustion (every call here does, via the while loop
  // below) already resets its own lastIndex to 0 on that final failed
  // match, but the explicit reset removes any doubt a future change could
  // introduce by returning out of the loop early.
  issueKeyPattern: RegExp | null;
  onIssueKey?: (key: string) => void;
  onOpenLink: (url: string) => void;
  inline: boolean;
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

// renderText splits one text leaf on \b<projectKey>-\d+\b, so parsers and
// toPlainText stay project-blind and this is the only place that knows an
// issue key when it sees one (a ruling, not an accident: the project key
// comes from the panel showing the issue, not from anything a parser could
// see in the field itself).
function renderText(value: string, opts: RenderOpts, keyPrefix: string): ReactNode {
  if (!opts.issueKeyPattern || !opts.onIssueKey) return value;
  const onIssueKey = opts.onIssueKey;
  const re = opts.issueKeyPattern;
  re.lastIndex = 0;
  const parts: ReactNode[] = [];
  let last = 0;
  let match: RegExpExecArray | null;
  let n = 0;
  while ((match = re.exec(value))) {
    if (match.index > last) parts.push(value.slice(last, match.index));
    const found = match[0];
    parts.push(
      <button key={`${keyPrefix}-k${n++}`} type="button" className="rich-issue-key" onClick={() => onIssueKey(found)}>
        {found}
      </button>
    );
    last = match.index + found.length;
  }
  if (parts.length === 0) return value;
  if (last < value.length) parts.push(value.slice(last));
  return parts;
}

function renderInlineList(nodes: Inline[], opts: RenderOpts, keyPrefix: string): ReactNode[] {
  return nodes.map((node, i) => (
    <Fragment key={`${keyPrefix}-${i}`}>{renderInlineNode(node, opts, `${keyPrefix}-${i}`)}</Fragment>
  ));
}

function renderInlineNode(node: Inline, opts: RenderOpts, key: string): ReactNode {
  switch (node.t) {
    case "text":
      return renderText(node.text, opts, key);
    case "mark": {
      const children = renderInlineList(node.children, opts, key);
      // D5: inline mode (the summary) renders a mark as plain text, never
      // bold or italic; Jira DC does not wiki- or Markdown-render that
      // field, so "2*3*4 items" must read exactly as typed.
      if (opts.inline) return children;
      const Tag = MARK_TAGS[node.mark];
      return <Tag>{children}</Tag>;
    }
    case "code":
      return <code className="rich-inline-code">{node.text}</code>;
    case "link": {
      const label = renderInlineList(node.children, opts, key);
      if (!isAllowedLink(node.href)) return label;
      return (
        <button
          type="button"
          role="link"
          className="rich-link"
          title={node.href}
          onClick={() => opts.onOpenLink(node.href)}
        >
          {label}
        </button>
      );
    }
    case "br":
      return <br />;
    case "image":
      // Constraint: no <img> anywhere under richtext/. An image macro is a
      // named placeholder, never fetched or drawn.
      return <span className="rich-image-placeholder">{`Image: ${node.name}`}</span>;
    default: {
      // Exhaustiveness guard: if a seventh Inline kind is ever added to
      // ast.ts without a case here, this assignment stops compiling
      // instead of silently rendering nothing and, for a link-shaped
      // addition, silently skipping isAllowedLink.
      const _never: never = node;
      return null;
    }
  }
}

function renderBlock(block: Block, opts: RenderOpts, key: string): ReactNode {
  switch (block.t) {
    case "p":
      return <p key={key}>{renderInlineList(block.children, opts, key)}</p>;
    case "h": {
      const Tag = HEADING_TAGS[block.level];
      return <Tag key={key}>{renderInlineList(block.children, opts, key)}</Tag>;
    }
    case "list": {
      const ListTag = block.ordered ? "ol" : "ul";
      return (
        <ListTag key={key}>
          {block.items.map((item, i) => (
            <li key={`${key}-${i}`}>
              {item.checked !== undefined && (
                <input type="checkbox" className="rich-task-checkbox" checked={item.checked} disabled readOnly />
              )}
              {item.children.map((child, j) => renderBlock(child, opts, `${key}-${i}-${j}`))}
            </li>
          ))}
        </ListTag>
      );
    }
    case "table":
      return (
        <div key={key} className="rich-table-scroll">
          <table>
            <tbody>
              {block.rows.map((row, i) => (
                <tr key={`${key}-${i}`}>
                  {row.map((cell, j) =>
                    cell.header ? (
                      <th key={`${key}-${i}-${j}`}>{renderInlineList(cell.children, opts, `${key}-${i}-${j}`)}</th>
                    ) : (
                      <td key={`${key}-${i}-${j}`}>{renderInlineList(cell.children, opts, `${key}-${i}-${j}`)}</td>
                    )
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      );
    case "codeblock":
      return (
        <pre key={key} className="rich-code-scroll">
          <code>{block.text}</code>
        </pre>
      );
    case "quote":
      return (
        <blockquote key={key}>{block.children.map((child, i) => renderBlock(child, opts, `${key}-${i}`))}</blockquote>
      );
    case "rule":
      return <hr key={key} />;
    case "panel":
      return (
        <div key={key} className="rich-panel">
          {block.title && <div className="rich-panel-title">{block.title}</div>}
          <div className="rich-panel-body">
            {block.children.map((child, i) => renderBlock(child, opts, `${key}-${i}`))}
          </div>
        </div>
      );
  }
}

// collectInline (D5, inline mode) keeps only paragraph and heading blocks,
// joined by a space, and drops every other block construct (lists, tables,
// code blocks, quotes, rules, panels): the summary is one line in Jira and
// nothing else needs to survive.
function collectInline(blocks: Block[]): Inline[] {
  const out: Inline[] = [];
  for (const block of blocks) {
    if (block.t !== "p" && block.t !== "h") continue;
    if (out.length > 0) out.push({ t: "text", text: " " });
    out.push(...block.children);
  }
  return out;
}

// RichText renders parsed wiki markup or Markdown as React elements: no
// raw-HTML injection prop, no direct DOM HTML assignment, no image element,
// no href attribute anywhere in this module (noHtml.test.ts reads the
// source to enforce it, so this comment cannot even name the three banned
// spellings without failing its own guard). A link is a
// <button role="link"> with no href; clicking it is the only way it ever
// reaches onOpenLink, and only http, https and mailto addresses render as
// one at all (isAllowedLink).
export function RichText({ text, format = "auto", inline = false, projectKey, onOpenLink, onIssueKey }: Props): ReactElement {
  // D4: parsing is memoized on (text, format), the two things that decide
  // its output; nothing else this component reads should re-run it.
  const parsed = useMemo(() => {
    const capped = text.length > RICH_TEXT_LIMIT ? text.slice(0, RICH_TEXT_LIMIT) : text;
    return parseRich(capped, format);
  }, [text, format]);

  // Built once per projectKey, not once per text leaf (item 3): the regex
  // itself depends only on projectKey, never on onIssueKey's identity, so
  // a caller passing a fresh onIssueKey closure every render (a common
  // React shape) does not force a rebuild.
  const issueKeyPattern = useMemo(
    () => (projectKey ? new RegExp(`\\b${escapeRegExp(projectKey)}-\\d+\\b`, "g") : null),
    [projectKey]
  );

  const truncated = text.length > RICH_TEXT_LIMIT;
  const opts: RenderOpts = { issueKeyPattern, onIssueKey, onOpenLink, inline };

  if (inline) {
    const nodes = collectInline(parsed.blocks);
    return (
      <span className="rich-text-inline">
        {truncated && <span className="rich-text-truncated">{SIZE_SENTENCE} </span>}
        {renderInlineList(nodes, opts, "i")}
      </span>
    );
  }

  return (
    <div className="rich-text">
      {truncated && <p className="rich-text-truncated">{SIZE_SENTENCE}</p>}
      {parsed.blocks.map((block, i) => renderBlock(block, opts, `b${i}`))}
    </div>
  );
}
