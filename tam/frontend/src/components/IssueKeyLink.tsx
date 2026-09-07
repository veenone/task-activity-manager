import { BrowserOpenURL, DRAFT_PREFIX, isDemoUrl } from "../api";

// browseUrl is where an issue lives on the instance the profile points at.
// Jira DC serves every issue at /browse/KEY, so the key is the whole route.
export function browseUrl(jiraUrl: string | undefined, key: string): string {
  const base = (jiraUrl ?? "").trim().replace(/\/+$/, "");
  if (base === "" || isDemoUrl(base) || key.startsWith(DRAFT_PREFIX)) return "";
  return `${base}/browse/${encodeURIComponent(key)}`;
}

interface Props {
  jiraUrl: string | undefined;
  issueKey: string;
  className?: string;
  children?: React.ReactNode;
}

// IssueKeyLink turns an issue key into a way out to the instance, the way
// XTM's detail header does. It opens in the user's own browser rather than
// the app's WebView: TAM is a companion to Jira, not a replacement for it,
// and a Jira page inside a 352px panel would be neither.
//
// It belongs to the detail panel's title and nowhere else. The grids' key
// cells are deliberately plain text: a row selects on click, so a link inside
// one puts two actions on the same pixels and the reader cannot tell which
// they are about to get.
//
// A key with nowhere to go renders as plain text: a draft has no page yet
// (Commit is what gives it one) and a demo profile has no server.
export function IssueKeyLink({ jiraUrl, issueKey, className, children }: Props) {
  const url = browseUrl(jiraUrl, issueKey);
  const label = children ?? issueKey;
  if (url === "") return <>{label}</>;
  return (
    <button
      type="button"
      className={`issue-key-link${className ? ` ${className}` : ""}`}
      title={`Open ${issueKey} in your browser`}
      onClick={(e) => {
        e.stopPropagation();
        BrowserOpenURL(url);
      }}
      onKeyDown={(e) => e.stopPropagation()}
    >
      {label}
      <span className="issue-key-ext" aria-hidden="true">↗</span>
    </button>
  );
}
