// The Confluence root page id field in Profile settings, read the way
// Confluence hands an id to a person: typed as a number, or inside a page
// address copied from the browser. Every sentence the form prints about the
// field lives here, so the rules are testable without rendering the form.

export const ROOT_ID_NOT_A_NUMBER = "The root page id is a number, such as 123456. You can also paste the page's address.";
export const ROOT_URL_WITHOUT_ID = "That address carries no page id. In Confluence, open the page's Page Information and copy that address, which ends in pageId=123456.";
export const ROOT_FIX_BEFORE_SAVE = "Fix the Confluence root page id under Confluence Rituals before saving.";

export interface RootPageInput {
  id: string;
  error: string;
}

const DIGITS = /^\d+$/;

export function readRootPageInput(value: string, confluenceUrl: string): RootPageInput {
  const text = value.trim();
  if (text === "") return { id: "", error: "" };
  // The demo space's own root is "demo-root", and nothing reaches a server.
  if (confluenceUrl.trim().toLowerCase() === "demo") return { id: text, error: "" };
  if (DIGITS.test(text)) return { id: text, error: "" };
  if (/^https?:\/\//i.test(text)) {
    let url: URL;
    try {
      url = new URL(text);
    } catch {
      return { id: "", error: ROOT_URL_WITHOUT_ID };
    }
    const pageId = url.searchParams.get("pageId") ?? "";
    return DIGITS.test(pageId) ? { id: pageId, error: "" } : { id: "", error: ROOT_URL_WITHOUT_ID };
  }
  return { id: "", error: ROOT_ID_NOT_A_NUMBER };
}
