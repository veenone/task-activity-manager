import { describe, it, expect } from "vitest";
import { jiraUrlError, normalizeJiraUrl, projectKeyError } from "./profileValidation";

const isDemo = (u: string) => /^demo(\b|[-:])/i.test(u);

describe("normalizeJiraUrl", () => {
  it("trims space and trailing slashes", () => {
    expect(normalizeJiraUrl("  https://jira.example.com/ ")).toBe("https://jira.example.com");
    expect(normalizeJiraUrl("https://jira.example.com///")).toBe("https://jira.example.com");
  });
});

describe("jiraUrlError", () => {
  it("accepts a blank field, the app's demo URLs and a full http(s) URL", () => {
    expect(jiraUrlError("", isDemo)).toBe("");
    expect(jiraUrlError("demo-pkcs", isDemo)).toBe("");
    expect(jiraUrlError("https://jira.example.com/", isDemo)).toBe("");
  });

  it("asks for a full URL, a web protocol and no spaces", () => {
    expect(jiraUrlError("jira.example.com", isDemo)).toBe(
      "Enter a full base URL, e.g. https://jira.example.com (or 'demo').",
    );
    expect(jiraUrlError("ftp://jira.example.com", isDemo)).toBe(
      "The URL must start with http:// or https:// (or be 'demo').",
    );
    expect(jiraUrlError("https://jira example.com", isDemo)).toBe("The URL must not contain spaces.");
  });

  it("uses the app's own idea of a demo URL", () => {
    expect(jiraUrlError("kiwi-demo", isDemo)).not.toBe("");
    expect(jiraUrlError("kiwi-demo", (u) => u === "kiwi-demo")).toBe("");
  });
});

describe("projectKeyError", () => {
  it("accepts a blank key and the documented shape", () => {
    expect(projectKeyError("")).toBe("");
    expect(projectKeyError("RND_P_4TFINT_05")).toBe("");
  });

  it("refuses a slash or a space, in words without an em dash", () => {
    const msg = projectKeyError("PLAT/");
    expect(msg).toBe(
      "Project key must start with a letter and use only letters, digits, and underscores. No spaces, slashes, or other special characters.",
    );
    expect(msg).not.toContain("—");
  });

  it("can leave the shape to Jira", () => {
    expect(projectKeyError("plat-1", false)).toBe("");
  });
});
