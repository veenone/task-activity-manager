import { useEffect, useRef, useState } from "react";
import { jiraUrlError, normalizeJiraUrl } from "@agile-suite/core";
import { DeleteBugConnection, GetBugConnection, SaveBugConnection, errMsg } from "../../api";
import { isXtmDemoUrl } from "./demoUrl";
import { useConfirm } from "../useConfirm";

interface Stored {
  id: string;
  url: string;
  projectKey: string;
  issueType: string;
  caCert: string;
  allowTls: boolean;
}

const NONE: Stored = { id: "", url: "", projectKey: "", issueType: "", caCert: "", allowTls: false };

// useBugTracker holds the Jira connection a Kiwi profile files bugs into
// (RND_P_4TFINT_05-359). Kiwi has no issue type of its own, so this is the
// only route for its defects. It is configured only on a Kiwi profile, never
// on a workspace connection; active says whether that is the case.
export function useBugTracker(profileId: string | undefined, active: boolean) {
  const { confirm } = useConfirm();
  // stored mirrors exactly what was last loaded or saved, kept apart from the
  // editable fields so a confirm can name the connection being removed after
  // the user cleared the fields, and so declining that confirm can put the
  // form back to what is actually stored.
  const [stored, setStored] = useState<Stored>(NONE);
  const [url, setUrl] = useState("");
  const [projectKey, setProjectKey] = useState("");
  const [issueType, setIssueType] = useState("");
  const [token, setToken] = useState("");
  const [caCert, setCaCert] = useState("");
  const [allowTls, setAllowTls] = useState(false);
  // Set when GetBugConnection itself fails, as opposed to nothing being
  // configured. It blocks Save: an unreadable connection must never be taken
  // for an unconfigured one, or Save would overwrite it.
  const [loadError, setLoadError] = useState("");

  function show(c: Stored) {
    setUrl(c.url);
    setProjectKey(c.projectKey);
    setIssueType(c.issueType);
    setCaCert(c.caCert);
    setAllowTls(c.allowTls);
  }

  // Load once per profile id, not on every Backend toggle, which would
  // overwrite what the user had typed. A new profile has no id yet, and
  // GetBugConnection answers a zero connection when none is configured.
  const loadedFor = useRef<string | null>(null);
  useEffect(() => {
    if (!active || !profileId) return;
    if (loadedFor.current === profileId) return;
    loadedFor.current = profileId;
    let cancelled = false;
    GetBugConnection(profileId)
      .then((c) => {
        if (cancelled) return;
        const s = { id: c.id, url: c.url, projectKey: c.projectKey, issueType: c.bugIssueType, caCert: c.caCert, allowTls: c.allowUntrustedTls };
        setStored(s);
        show(s);
        setLoadError("");
      })
      .catch((e) => {
        if (cancelled) return;
        // Allow a retry: an id that never loaded should not stay sticky.
        loadedFor.current = null;
        setLoadError(errMsg(e));
      });
    return () => {
      cancelled = true;
    };
  }, [active, profileId]);

  const urlError = active ? jiraUrlError(url, isXtmDemoUrl) : "";
  // Exactly one of URL and project key filled is an edit in progress, not a
  // removal, so Save waits rather than guessing what the user meant.
  const partial = active && (url.trim() !== "") !== (projectKey.trim() !== "");
  const valid = !partial && urlError === "" && (!active || loadError === "");

  // save writes the connection for a profile that was just saved, or removes
  // it when both fields were cleared and the user confirms. The profile is
  // already saved by then, so a failure comes back as a warning to show
  // beside the success rather than as an error.
  async function save(id: string): Promise<string | void> {
    if (!active) return;
    const trackerUrl = normalizeJiraUrl(url);
    const trackerKey = projectKey.trim();
    try {
      if (trackerUrl !== "" && trackerKey !== "") {
        // A blank token keeps the stored one on update.
        const c = await SaveBugConnection(id, trackerUrl, trackerKey, issueType.trim(), token.trim(), caCert.trim(), allowTls);
        setStored({ id: c.id, url: c.url, projectKey: c.projectKey, issueType: c.bugIssueType, caCert: c.caCert, allowTls: c.allowUntrustedTls });
      } else if (trackerUrl === "" && trackerKey === "" && stored.id !== "") {
        // Both cleared with a connection on record is the one clear "turn
        // bug routing off". It also deletes the stored credential, so ask.
        const ok = await confirm({
          title: "Remove the bug tracker connection?",
          message: `This stops bugs from being filed at ${stored.url} (project ${stored.projectKey}) and deletes its stored credential.`,
          confirmLabel: "Remove connection",
          danger: true,
        });
        if (ok) {
          await DeleteBugConnection(id);
          setStored(NONE);
        } else {
          show(stored);
        }
      }
    } catch (e) {
      return `Profile saved, but the bug tracker connection could not be saved: ${errMsg(e)}`;
    }
  }

  return {
    hasStored: stored.id !== "", url, setUrl, projectKey, setProjectKey, issueType, setIssueType,
    token, setToken, caCert, setCaCert, allowTls, setAllowTls, loadError, urlError, partial, valid, save,
  };
}

export type BugTracker = ReturnType<typeof useBugTracker>;
