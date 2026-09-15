const URL_ATTRIBUTES = new Set(["href", "src", "xlink:href", "action"]);
const SAFE_SCHEMES = new Set(["http", "https", "mailto"]);

// compact drops ASCII control characters, space, and DEL, the characters the
// browser removes from a URL before it resolves the scheme.
function compact(value: string): string {
  let out = "";
  for (const ch of value) {
    const code = ch.charCodeAt(0);
    if (code > 0x20 && code !== 0x7f) out += ch;
  }
  return out;
}

// safeUrl allows only http, https, mailto and a relative URL. It reads the
// scheme after compact, because "java&#9;script:" is a javascript: URL to the
// browser, and a pattern tested against the raw value never saw it as one.
function safeUrl(value: string): boolean {
  const scheme = /^([a-z][a-z0-9+.-]*):/i.exec(compact(value));
  return !scheme || SAFE_SCHEMES.has(scheme[1].toLowerCase());
}

// isAllowedLink is what the editor's link box accepts: an address whose
// scheme, read after compact the way safeUrl reads it, is http, https or
// mailto. Unlike safeUrl it refuses a relative address, which on a page that
// lives in Confluence and in TAM at once points nowhere useful.
export function isAllowedLink(value: string): boolean {
  const scheme = /^([a-z][a-z0-9+.-]*):/i.exec(compact(value));
  return !!scheme && SAFE_SCHEMES.has(scheme[1].toLowerCase());
}

// A style can load a URL or, in old engines, run script. A backslash is
// refused with them, since a CSS escape can spell either word.
function unsafeStyle(value: string): boolean {
  const flat = compact(value).toLowerCase();
  return flat.includes("url(") || flat.includes("expression(") || flat.includes(String.fromCharCode(0x5c));
}

export function sanitizeHtml(value: string): string {
  if (typeof DOMParser === "undefined") return "";
  const document = new DOMParser().parseFromString(value, "text/html");
  document.querySelectorAll("script, iframe, object, embed, form, link, meta, style").forEach((node) => node.remove());
  document.querySelectorAll("*").forEach((node) => {
    [...node.attributes].forEach((attribute) => {
      const name = attribute.name.toLowerCase();
      if (name.startsWith("on") || name === "srcdoc") node.removeAttribute(attribute.name);
      else if (URL_ATTRIBUTES.has(name) && !safeUrl(attribute.value)) node.removeAttribute(attribute.name);
      else if (name === "style" && unsafeStyle(attribute.value)) node.removeAttribute(attribute.name);
    });
  });
  return document.body.innerHTML;
}
