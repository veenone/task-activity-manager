import { compactUrl, isAllowedLink } from "@agile-suite/core";

const URL_ATTRIBUTES = new Set(["href", "src", "xlink:href", "action"]);
const SAFE_SCHEMES = new Set(["http", "https", "mailto"]);

// isAllowedLink now lives in frontend/core/src/lib/links.ts, one definition
// shared with the rich text renderer's link nodes. Re-exported so
// useRitualToolbar's import (and, before this move, this file's own test)
// keeps working unchanged.
export { isAllowedLink };

// safeUrl allows only http, https, mailto and a relative URL. It reads the
// scheme after compactUrl, because "java&#9;script:" is a javascript: URL to
// the browser, and a pattern tested against the raw value never saw it as
// one. Unlike isAllowedLink it allows a relative address, which is what a
// Confluence page's own links legitimately carry.
function safeUrl(value: string): boolean {
  const scheme = /^([a-z][a-z0-9+.-]*):/i.exec(compactUrl(value));
  return !scheme || SAFE_SCHEMES.has(scheme[1].toLowerCase());
}

// A style can load a URL or, in old engines, run script. A backslash is
// refused with them, since a CSS escape can spell either word.
function unsafeStyle(value: string): boolean {
  const flat = compactUrl(value).toLowerCase();
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
