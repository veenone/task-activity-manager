// Validation for the Jira fields of a profile, shared by both apps'
// ProfileForm. Each app passes its own idea of a demo URL, since XTM also
// accepts its Kiwi demo forms.

// normalizeJiraUrl trims surrounding whitespace and strips trailing slashes so
// the stored base URL is clean: "https://jira.example.com/" or a pasted
// ".../secure/Dashboard.jspa " reduces to the bare base.
export function normalizeJiraUrl(url: string): string {
  return url.trim().replace(/[\s/]+$/, "");
}

// jiraUrlError validates the Jira base URL. A URL the app counts as a demo is
// allowed; otherwise it must be a well-formed http(s) URL with a host and no
// spaces. Blank is left to the Save button, which is disabled without a URL.
export function jiraUrlError(url: string, isDemo: (url: string) => boolean): string {
  const u = normalizeJiraUrl(url);
  if (u === "" || isDemo(u)) return "";
  if (/\s/.test(u)) return "The URL must not contain spaces.";
  let parsed: URL;
  try {
    parsed = new URL(u);
  } catch {
    return "Enter a full base URL, e.g. https://jira.example.com (or 'demo').";
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    return "The URL must start with http:// or https:// (or be 'demo').";
  }
  if (!parsed.hostname) {
    return "The URL must include a host, e.g. https://jira.example.com.";
  }
  return "";
}

// projectKeyError checks a Jira project key against the shape Jira documents:
// a letter first, then letters, digits and underscores (e.g. RND_P_4TFINT_05);
// any case is accepted and upper-cased on input. That shape is a convention of
// an instance rather than a rule, so an app can switch the check off and let
// Jira decide (#99). Blank is left to the Save button.
export function projectKeyError(key: string, checkPattern = true): string {
  const k = key.trim();
  if (k === "" || !checkPattern) return "";
  if (!/^[A-Z][A-Z0-9_]+$/.test(k)) {
    return "Project key must start with a letter and use only letters, digits, and underscores. No spaces, slashes, or other special characters.";
  }
  return "";
}
