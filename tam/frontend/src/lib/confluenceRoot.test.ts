import { describe, expect, it } from "vitest";
import { ROOT_ID_NOT_A_NUMBER, ROOT_URL_WITHOUT_ID, readRootPageInput } from "./confluenceRoot";

const CONF = "https://confluence.example.com";

describe("readRootPageInput", () => {
  it("accepts a page id, trimmed", () => {
    expect(readRootPageInput(" 653264152 ", CONF)).toEqual({ id: "653264152", error: "" });
  });

  it("reads the pageId out of a pasted page address", () => {
    expect(readRootPageInput(`${CONF}/pages/viewpage.action?pageId=653264152`, CONF)).toEqual({ id: "653264152", error: "" });
    expect(readRootPageInput(`${CONF}/pages/viewinfo.action?pageId=42&src=contextnavpagetreemode`, "")).toEqual({ id: "42", error: "" });
  });

  it("refuses an address that carries no page id", () => {
    expect(readRootPageInput(`${CONF}/display/TEAM/Rituals`, CONF)).toEqual({ id: "", error: ROOT_URL_WITHOUT_ID });
    expect(readRootPageInput(`${CONF}/pages/viewpage.action?pageId=abc`, CONF)).toEqual({ id: "", error: ROOT_URL_WITHOUT_ID });
  });

  it("refuses text that is not a number", () => {
    for (const value of ["Team rituals", "12a", "-5", "1.5"]) {
      expect(readRootPageInput(value, CONF)).toEqual({ id: "", error: ROOT_ID_NOT_A_NUMBER });
    }
  });

  it("leaves a blank field alone", () => {
    expect(readRootPageInput("   ", CONF)).toEqual({ id: "", error: "" });
  });

  it("accepts the demo space's own root id", () => {
    expect(readRootPageInput("demo-root", "Demo")).toEqual({ id: "demo-root", error: "" });
  });
});
