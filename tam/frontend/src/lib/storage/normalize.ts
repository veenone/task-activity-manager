import { escapeAttr, escapeText, isBlank, parseXml } from "./xml";

// normalizeStorage is the equality the round-trip contract is stated in:
// whitespace between elements, attribute order, and self-closing form do not
// count, and nothing else is forgiven. It exists for tests and for nothing
// the app decides on.
export function normalizeStorage(body: string): string {
  const root = parseXml(body);
  return root ? children(root) : `unparseable:${body}`;
}

function children(el: Element): string {
  const nodes = Array.from(el.childNodes);
  const hasElement = nodes.some((c) => c.nodeType === Node.ELEMENT_NODE);
  return nodes.map((c) => {
    switch (c.nodeType) {
      case Node.ELEMENT_NODE: {
        const e = c as Element;
        const attrs = Array.from(e.attributes).map((a) => `${a.name}="${escapeAttr(a.value)}"`).sort().join(" ");
        return `<${e.nodeName}${attrs ? ` ${attrs}` : ""}>${children(e)}</${e.nodeName}>`;
      }
      case Node.CDATA_SECTION_NODE:
        return `<![CDATA[${c.textContent ?? ""}]]>`;
      case Node.TEXT_NODE: {
        const text = c.textContent ?? "";
        return hasElement && isBlank(text) ? "" : escapeText(text);
      }
      case Node.COMMENT_NODE:
        return `<!--${c.textContent ?? ""}-->`;
    }
    return "";
  }).join("");
}
