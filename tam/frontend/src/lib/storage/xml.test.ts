import { describe, expect, it } from "vitest";
import { escapeAttr, escapeText, isBlank, outerXml, parseXml, toNumericEntities } from "./xml";
import { normalizeStorage } from "./normalize";

describe("storage XML", () => {
  it("rewrites HTML named entities as numeric references and leaves XML's own five alone", () => {
    expect(toNumericEntities("a&nbsp;b &mdash; &amp; &lt; &gt; &quot; &apos;")).toBe("a&#160;b &#8212; &amp; &lt; &gt; &quot; &apos;");
  });

  it("leaves an entity nobody knows, so the parse fails rather than guessing", () => {
    expect(toNumericEntities("&bogus;")).toBe("&bogus;");
    expect(parseXml("<p>&bogus;</p>")).toBeNull();
  });

  it("reads the ac: and ri: prefixes Confluence never declares", () => {
    const root = parseXml('<ac:link><ri:user ri:userkey="u1"/></ac:link>');
    expect(root?.firstElementChild?.nodeName).toBe("ac:link");
  });

  it("refuses malformed XML", () => {
    expect(parseXml("<p>unclosed")).toBeNull();
  });

  it("writes a fragment back without the declarations the wrapper added, CDATA intact", () => {
    const xml = '<ac:structured-macro ac:name="code"><ac:plain-text-body><![CDATA[if a < b && c > d {}]]></ac:plain-text-body></ac:structured-macro>';
    const root = parseXml(xml)!;
    expect(outerXml(root.firstChild!)).toBe(xml);
  });

  it("strips a namespace declaration only from a start tag, never from text or CDATA that merely looks like one", () => {
    const withCdata = '<ac:structured-macro ac:name="code"><ac:plain-text-body><![CDATA[<ac:x xmlns:ac="http://atlassian.com/content"/>]]></ac:plain-text-body></ac:structured-macro>';
    const root = parseXml(withCdata)!;
    expect(outerXml(root.firstChild!)).toBe(withCdata);

    const withText = parseXml('<p>write xmlns:ac="u" here</p>')!;
    expect(outerXml(withText.firstChild!)).toBe('<p>write xmlns:ac="u" here</p>');
  });

  it("escapes text and attribute values", () => {
    expect(escapeText(`a & <b> "c"`)).toBe(`a &amp; &lt;b&gt; "c"`);
    expect(escapeAttr(`a & "c"`)).toBe("a &amp; &quot;c&quot;");
  });

  it("treats only ASCII whitespace as blank, so a non-breaking space is content", () => {
    expect(isBlank(" \n\t")).toBe(true);
    expect(isBlank(" ")).toBe(false);
  });

  it("normalises whitespace between elements, attribute order and self-closing form, and nothing else", () => {
    expect(normalizeStorage('<p b="2" a="1">x</p>\n  <br/>')).toBe(normalizeStorage('<p a="1" b="2">x</p><br></br>'));
    expect(normalizeStorage("<p>a b</p>")).not.toBe(normalizeStorage("<p>ab</p>"));
    expect(normalizeStorage("<p>&nbsp;</p>")).toBe(normalizeStorage("<p> </p>"));
  });
});
