import { useState } from "react";
import type { Issue, Link } from "../api";
import { useIssueDetail, useLinkedTests } from "../queries/issues";
import { formatWhen } from "../lib/format";
import { TypeChip } from "./TypeChip";
import { EditableFields } from "./EditableFields";
import { ActivityTab } from "./ActivityTab";

type Tab = "details" | "links" | "tests" | "activity";

const TABS: { id: Tab; label: string }[] = [
  { id: "details", label: "Details" },
  { id: "links", label: "Links" },
  { id: "tests", label: "Tests" },
  { id: "activity", label: "Activity" },
];

interface Props {
  profileId: string;
  issue: Issue;
  onClose: () => void;
}

// IssueDetailPanel shows one issue beside the grid. The grid row's fields
// render at once; the description, links, and linked tests load through the
// backend's detail cache. Nothing here writes; the actions arrive in plan 1b.
export function IssueDetailPanel({ profileId, issue, onClose }: Props) {
  const [tab, setTab] = useState<Tab>("details");
  const detail = useIssueDetail(profileId, issue.key);
  const tests = useLinkedTests(profileId, issue.key);

  return (
    <aside className="detail-panel" aria-labelledby="detail-title">
      <div className="detail-head">
        <h2 id="detail-title" className="detail-key">{issue.key}</h2>
        <TypeChip type={issue.type} />
        {issue.draft && <span className="chip chip-draft">Draft</span>}
        <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close">×</button>
      </div>
      <p className="detail-summary">{issue.summary}</p>

      <div className="tabs" role="tablist" aria-label="Issue sections">
        {TABS.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            id={`tab-${t.id}`}
            aria-selected={tab === t.id}
            aria-controls={`panel-${t.id}`}
            className={`tab${tab === t.id ? " tab-active" : ""}`}
            onClick={() => setTab(t.id)}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === "details" && (
        <div role="tabpanel" id="panel-details" aria-labelledby="tab-details" className="tab-panel">
          {issue.draft && (
            <p className="muted small detail-note">Commit creates this issue in Jira and gives it a real key.</p>
          )}
          <dl className="field-list">
            <dt>Status</dt><dd>{issue.status || "-"}</dd>
            <dt>Sprint</dt><dd>{issue.sprintName || "-"}</dd>
            <dt>{issue.type === "epic" ? "Parent" : "Epic"}</dt><dd>{issue.parentKey ? <span className="accent-text">{issue.parentKey}</span> : "-"}</dd>
            <dt>Updated</dt><dd>{formatWhen(issue.updated) || "-"}</dd>
          </dl>
          <div className="detail-section-head">
            <h3>Fields</h3>
            <button type="button" className="btn btn-ghost" onClick={() => void detail.refetch()} disabled={detail.isFetching}>
              {detail.isFetching ? "Refreshing" : "Refresh"}
            </button>
          </div>
          {detail.isError && (
            <p className="error-text" data-testid="detail-error">
              Could not load the details: {detail.error.message}{" "}
              <button type="button" className="btn btn-ghost" onClick={() => void detail.refetch()}>Retry</button>
            </p>
          )}
          <EditableFields
            profileId={profileId}
            issue={issue}
            description={detail.data?.description ?? ""}
            descriptionReady={detail.isSuccess}
          />
        </div>
      )}

      {tab === "links" && (
        <div role="tabpanel" id="panel-links" aria-labelledby="tab-links" className="tab-panel">
          <div className="detail-section-head">
            <h3>Links</h3>
            <button type="button" className="btn btn-ghost" onClick={() => void detail.refetch()} disabled={detail.isFetching}>
              {detail.isFetching ? "Refreshing" : "Refresh"}
            </button>
          </div>
          {detail.isPending ? (
            <p className="muted">Loading links</p>
          ) : detail.isError ? (
            <p className="error-text" data-testid="links-error">
              Could not load the links: {detail.error.message}{" "}
              <button type="button" className="btn btn-ghost" onClick={() => void detail.refetch()}>Retry</button>
            </p>
          ) : detail.data.links.length === 0 ? (
            <p className="muted">No links.</p>
          ) : (
            <LinkGroups links={detail.data.links} />
          )}
        </div>
      )}

      {tab === "tests" && (
        <div role="tabpanel" id="panel-tests" aria-labelledby="tab-tests" className="tab-panel">
          <div className="detail-section-head">
            <h3>Covered by tests</h3>
            {tests.data && tests.data.length > 0 && (
              <span className="muted small">via XTM, link: {tests.data[0].linkType}</span>
            )}
            <button type="button" className="btn btn-ghost" onClick={() => void tests.refetch()} disabled={tests.isFetching}>
              {tests.isFetching ? "Refreshing" : "Refresh"}
            </button>
          </div>
          {tests.isPending ? (
            <p className="muted">Loading tests</p>
          ) : tests.isError ? (
            <p className="error-text" data-testid="tests-error">
              Could not load the linked tests: {tests.error.message}{" "}
              <button type="button" className="btn btn-ghost" onClick={() => void tests.refetch()}>Retry</button>
            </p>
          ) : tests.data.length === 0 ? (
            <p className="muted">No linked tests.</p>
          ) : (
            <ul className="linked-list">
              {tests.data.map((t) => (
                <li key={t.key} className="linked-row">
                  <span className="accent-text linked-key">{t.key}</span>
                  <span>{t.summary}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {tab === "activity" && <ActivityTab profileId={profileId} issueKey={issue.key} />}
    </aside>
  );
}

// LinkGroups lists links grouped by type, then direction, in the order the
// store returns them.
function LinkGroups({ links }: { links: Link[] }) {
  const groups = new Map<string, Link[]>();
  for (const l of links) {
    const k = `${l.type} (${l.direction})`;
    groups.set(k, [...(groups.get(k) ?? []), l]);
  }
  return (
    <div className="link-groups">
      {[...groups.entries()].map(([label, items]) => (
        <div key={label} className="link-group">
          <h3 className="link-group-title">{label.replace(/ \((inward|outward)\)$/, "")} <span className="muted small">{label.match(/\((inward|outward)\)$/)?.[1]}</span></h3>
          <ul className="linked-list">
            {items.map((l) => (
              <li key={`${l.direction}-${l.key}`} className="linked-row">
                <span className="accent-text linked-key">{l.key}</span>
                <span>{l.summary}</span>
                <span className="muted small">{l.issueType}</span>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  );
}
