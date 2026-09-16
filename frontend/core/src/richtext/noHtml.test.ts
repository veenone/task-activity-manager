/// <reference types="vite/client" />
import { describe, expect, it } from "vitest";

// This is the bundle's security promise, enforced by reading the actual
// source under richtext/ rather than by trusting a review: none of it may
// build raw HTML or point a real element at a URL. RichText renders every
// link as a <button role="link"> with no href and calls onOpenLink itself.
const files = import.meta.glob("./**/*.{ts,tsx}", { query: "?raw", import: "default", eager: true }) as Record<
  string,
  string
>;

const sources = Object.entries(files).filter(([path]) => !path.includes(".test."));

describe("no HTML injection under richtext/", () => {
  it("found richtext source files to check", () => {
    expect(sources.length).toBeGreaterThan(0);
  });

  it.each(sources)("%s carries no dangerouslySetInnerHTML, innerHTML or href=", (path, source) => {
    expect(source).not.toMatch(/dangerouslySetInnerHTML/);
    expect(source).not.toMatch(/\.innerHTML/);
    expect(source).not.toMatch(/href=/);
  });
});
