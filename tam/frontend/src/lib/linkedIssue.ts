import { useState } from "react";
import { errMsg, useConfirm, useNotice } from "@agile-suite/core";
import { BrowserOpenURL, GetCachedIssue } from "../api";
import type { Issue } from "../api";
import { browseUrl } from "../components/IssueKeyLink";

// useLinkedIssue lets the detail panel open a linked or parent issue in place,
// the way XTM opens related items, instead of sending the reader to Jira
// (#530). It returns the issue the panel should show: the caller's selection,
// or the issue opened from it, until the caller selects something else.
//
// A key the cache does not hold says so. Where the profile has a Jira to go
// to, the dialog offers it; a demo profile or a draft key has none.
export function useLinkedIssue(profileId: string, issue: Issue, jiraUrl: string | undefined) {
  const [opened, setOpened] = useState<{ from: string; issue: Issue } | null>(null);
  const { confirm } = useConfirm();
  const { notice } = useNotice();
  const shown = opened && opened.from === issue.key ? opened.issue : issue;

  async function open(key: string) {
    let found: Issue | null;
    try {
      found = await GetCachedIssue(profileId, key);
    } catch (e) {
      await notice({ title: `Could not open ${key}`, message: errMsg(e), tone: "error" });
      return;
    }
    if (found) {
      setOpened({ from: issue.key, issue: found });
      return;
    }
    const title = `${key} is not in TAM`;
    const url = browseUrl(jiraUrl, key);
    if (!url) {
      await notice({ title, message: "TAM has not synced this issue, and this profile has no Jira to open it on." });
      return;
    }
    const go = await confirm({
      title,
      message: "TAM has not synced this issue. Open it in Jira instead?",
      confirmLabel: "Open in Jira",
      danger: false,
    });
    if (go) BrowserOpenURL(url);
  }

  return { shown, open };
}
