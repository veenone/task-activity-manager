import { describe, expect, it } from "vitest";
import { isAllowedLink, sanitizeHtml } from "./sanitizeHtml";

function hrefOf(html: string, selector = "a"): string | null {
  const host = document.createElement("div");
  host.innerHTML = sanitizeHtml(html);
  return host.querySelector(selector)?.getAttribute("href") ?? null;
}

describe("sanitizeHtml", () => {
  // The browser strips a tab or newline from a URL before it reads the
  // scheme, so a pattern tested against the raw value let these through.
  it("removes a javascript: href hidden behind a control character", () => {
    expect(hrefOf(`<a href="java&#9;script:alert(1)">x</a>`)).toBeNull();
    expect(hrefOf(`<a href="&#10; javascript:alert(1)">x</a>`)).toBeNull();
  });

  it("removes an xlink:href that is not http, https, mailto or relative", () => {
    const host = document.createElement("div");
    host.innerHTML = sanitizeHtml(`<svg><a xlink:href="javascript:alert(1)"><text>x</text></a></svg>`);
    const anchor = host.querySelector("a")!;
    expect([...anchor.attributes].map((a) => a.name)).not.toContain("xlink:href");
  });

  it("removes other schemes from src and action too", () => {
    const host = document.createElement("div");
    host.innerHTML = sanitizeHtml(`<img src="data:text/html,x"><button action="vbscript:x">b</button>`);
    expect(host.querySelector("img")!.hasAttribute("src")).toBe(false);
    expect(host.querySelector("button")!.hasAttribute("action")).toBe(false);
  });

  it("keeps https, mailto and relative links", () => {
    expect(hrefOf(`<a href="https://example.com/x">x</a>`)).toBe("https://example.com/x");
    expect(hrefOf(`<a href="mailto:team@example.com">x</a>`)).toBe("mailto:team@example.com");
    expect(hrefOf(`<a href="/wiki/x#y">x</a>`)).toBe("/wiki/x#y");
    expect(hrefOf(`<a href="#top">x</a>`)).toBe("#top");
  });

  it("removes a style that loads a url or runs an expression", () => {
    const host = document.createElement("div");
    host.innerHTML = sanitizeHtml(`<p style="background: URL (x.png)">a</p><p style="width: expression(alert(1))">b</p><p style="color: red">c</p>`);
    const [a, b, c] = [...host.querySelectorAll("p")];
    expect(a.hasAttribute("style")).toBe(false);
    expect(b.hasAttribute("style")).toBe(false);
    expect(c.getAttribute("style")).toBe("color: red");
  });
});

describe("isAllowedLink", () => {
  it("allows http, https and mailto addresses, in any case", () => {
    for (const href of ["https://example.com", "http://example.com/a?b=1", "MAILTO:team@example.com", "HTTPS://EXAMPLE.COM"]) {
      expect(isAllowedLink(href)).toBe(true);
    }
  });

  it("refuses other schemes, disguised ones, and relative addresses", () => {
    for (const href of ["javascript:alert(1)", "java\tscript:alert(1)", " javascript:alert(1)", "data:text/html,x", "ftp://example.com", "example.com", "/wiki/x", ""]) {
      expect(isAllowedLink(href)).toBe(false);
    }
  });
});
