import { useEffect, useMemo, useState } from "react";
import { ListBugsWithTests, BrowserOpenURL, errMsg } from "../api";
import type { BugWithTests } from "../api";
import { Pager } from "./Pager";

interface Props {
  profileId: string;
  refreshKey: number;
  jiraUrl: string;
  onOpenTest: (testKey: string) => void;
}

// BugsPanel lists every bug linked to the profile's tests, with the tests each
// affects. Bug keys open in the browser; test keys open the test detail.
export function BugsPanel({ profileId, refreshKey, jiraUrl, onOpenTest }: Props) {
  const [bugs, setBugs] = useState<BugWithTests[]>([]);
  const [filter, setFilter] = useState("");
  const [error, setError] = useState("");
  const [page, setPage] = useState(0); // 0-based
  const [pageSize, setPageSize] = useState(15);

  useEffect(() => {
    if (!profileId) return;
    let cancelled = false;
    ListBugsWithTests(profileId)
      .then((bs) => {
        if (!cancelled) setBugs(bs ?? []);
      })
      .catch((e) => {
        if (!cancelled) setError(errMsg(e));
      });
    return () => {
      cancelled = true;
    };
  }, [profileId, refreshKey]);

  const isDemo = /^(demo$|demo:|mock:)/i.test((jiraUrl ?? "").trim());
  const canLink = !!jiraUrl && !isDemo;
  function openBug(key: string) {
    const base = (jiraUrl ?? "").trim().replace(/\/+$/, "");
    if (base && canLink && !key.startsWith("NEW-")) BrowserOpenURL(`${base}/browse/${key}`);
  }

  const shown = useMemo(() => {
    const f = filter.trim().toLowerCase();
    if (!f) return bugs;
    return bugs.filter(
      (b) =>
        b.key.toLowerCase().includes(f) ||
        b.summary.toLowerCase().includes(f) ||
        b.projectKey.toLowerCase().includes(f) ||
        b.status.toLowerCase().includes(f),
    );
  }, [bugs, filter]);

  // Reset to the first page whenever the data source or the filter changes, so
  // a narrowed result set never leaves us stranded on an empty page.
  useEffect(() => {
    setPage(0);
  }, [profileId, refreshKey, filter]);

  const totalPages = Math.max(1, Math.ceil(shown.length / pageSize));
  const safePage = Math.min(Math.max(0, page), totalPages - 1);
  const paged = shown.slice(safePage * pageSize, safePage * pageSize + pageSize);

  return (
    <div className="bugs-panel">
      {error && <div className="error-text">{error}</div>}
      <input
        className="search"
        placeholder="Filter bugs by key, summary, project, status…"
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
      />
      {shown.length === 0 ? (
        <p className="muted">
          {bugs.length === 0
            ? "No bugs linked to this profile's tests. File one from a failed test in a Test Execution, or sync a demo profile."
            : "No bugs match the filter."}
        </p>
      ) : (
        <table className="board-table bugs-table">
          <thead>
            <tr>
              <th>Bug</th>
              <th>Project</th>
              <th>Summary</th>
              <th>Status</th>
              <th>Priority</th>
              <th>Affects</th>
            </tr>
          </thead>
          <tbody>
            {paged.map((b) => (
              <tr key={b.key}>
                <td>
                  {canLink && !b.key.startsWith("NEW-") ? (
                    <button className="mono bug-link-key" onClick={() => openBug(b.key)} title={`Open ${b.key} in Jira`}>
                      {b.key}
                    </button>
                  ) : (
                    <span className="mono">{b.key}</span>
                  )}
                </td>
                <td className="muted">{b.projectKey}</td>
                <td>{b.summary}</td>
                <td>{b.status && <span className="status-pill">{b.status}</span>}</td>
                <td>{b.priority}</td>
                <td>
                  {b.testKeys.map((tk, i) => (
                    <span key={tk}>
                      {i > 0 && ", "}
                      <button className="mono bug-link-key" onClick={() => onOpenTest(tk)} title={`Open ${tk}`}>
                        {tk}
                      </button>
                    </span>
                  ))}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {shown.length > 0 && (
        <Pager
          page={safePage}
          pageSize={pageSize}
          total={shown.length}
          onPage={setPage}
          onPageSize={(n) => {
            setPageSize(n);
            setPage(0);
          }}
        />
      )}
    </div>
  );
}
