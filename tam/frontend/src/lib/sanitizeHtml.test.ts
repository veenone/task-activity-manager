import { describe, expect, it } from "vitest";
import { sanitizeHtml } from "./sanitizeHtml";

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

  it("drops SVG, its links and its animation values", () => {
    const out = sanitizeHtml(
      `<svg><a xlink:href="javascript:alert(1)"><text>x</text></a>` +
        `<a><animate attributeName="href" values="javascript:alert(1)"/><text>y</text></a></svg><p>after</p>`,
    );
    expect(out).toBe("<p>after</p>");
  });

  it("removes other schemes from src, and drops a form control to its text", () => {
    expect(sanitizeHtml(`<img src="data:text/html,x"><button action="vbscript:x">b</button>`)).toBe("<img>b");
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

  it("drops a base element, so relative links cannot be redirected", () => {
    // After body content the parser keeps <base> in the body, not the head.
    expect(sanitizeHtml(`<p>a</p><base href="https://evil.example/"><p><a href="/wiki/x">x</a></p>`)).toBe(
      `<p>a</p><p><a href="/wiki/x">x</a></p>`,
    );
  });

  it("removes positioning and other layout styles, keeping colour and alignment", () => {
    const host = document.createElement("div");
    host.innerHTML = sanitizeHtml(
      `<div style="position: fixed; inset: 0; z-index: 9999; color: rgb(255, 0, 0)">a</div>` +
        `<p style="text-align: center; transform: scale(9)">b</p>`,
    );
    expect(host.querySelector("div")!.getAttribute("style")).toBe("color: rgb(255, 0, 0)");
    expect(host.querySelector("p")!.getAttribute("style")).toBe("text-align: center");
  });

  it("drops script and event-handler attributes and unwraps unknown tags", () => {
    expect(sanitizeHtml(`<p onclick="alert(1)">a<script>alert(2)</script></p><img src=x onerror="alert(3)">`)).toBe(
      `<p>a</p><img src="x">`,
    );
    expect(sanitizeHtml(`<custom-el data-x="1"><b id="k" class="modal">bold</b></custom-el>`)).toBe("<b>bold</b>");
  });

  it("keeps the formatting a ritual page carries", () => {
    const page =
      `<h2>Notes</h2><p><strong>b</strong> <em>i</em> <u>u</u> <s>s</s> <code>c</code></p>` +
      `<ul><li>one</li></ul><ol start="2"><li>two</li></ol><blockquote><p>q</p></blockquote><hr>` +
      `<table><tbody><tr><th colspan="2">h</th></tr><tr><td rowspan="1">c</td></tr></tbody></table>`;
    expect(sanitizeHtml(page)).toBe(page);
  });
});
