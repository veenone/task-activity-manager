const SAFE_SCHEMES = new Set(["http", "https", "mailto"]);

// compactUrl drops ASCII control characters, space, and DEL, the characters
// the browser removes from a URL before it resolves the scheme. Was
// sanitizeHtml's private "compact"; moved here so the rich text renderer and
// the ritual editor's sanitizer read the same URL the same way.
export function compactUrl(value: string): string {
  let out = "";
  for (const ch of value) {
    const code = ch.charCodeAt(0);
    if (code > 0x20 && code !== 0x7f) out += ch;
  }
  return out;
}

// isAllowedLink is what the ritual editor's link box and the rich text
// renderer's link nodes accept: an address whose scheme, read after
// compactUrl (because "java&#9;script:" is a javascript: URL to the
// browser, and a pattern tested against the raw value never saw it as
// one), is http, https or mailto. Unlike sanitizeHtml's own safeUrl it
// refuses a relative address, which on a rich text field rendered with no
// page of its own points nowhere useful.
export function isAllowedLink(value: string): boolean {
  const scheme = /^([a-z][a-z0-9+.-]*):/i.exec(compactUrl(value));
  return !!scheme && SAFE_SCHEMES.has(scheme[1].toLowerCase());
}
