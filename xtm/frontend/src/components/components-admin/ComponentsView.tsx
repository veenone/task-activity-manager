import { useEffect, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useProfile } from "../../contexts/ProfileContext";
import {
  useComponentCounts,
  useComponentTests,
  useProjectComponents,
} from "../../queries/components";
import { keys } from "../../queries/keys";
import {
  ComponentIssueCount,
  CreateComponent,
  DeleteComponent,
  UpdateComponent,
  errMsg,
} from "../../api";
import type { ComponentInput, ProjectComponent } from "../../api";
import { Pager } from "../Pager";
import { SortControl } from "../SortControl";
import { ComponentFormModal } from "./ComponentFormModal";
import { DeleteComponentModal } from "./DeleteComponentModal";

type Dialog =
  | { kind: "none" }
  | { kind: "new" }
  | { kind: "edit"; component: ProjectComponent }
  | { kind: "delete"; component: ProjectComponent; issueCount: number };

// The page sizes this view offers. Ten to start, then up in tens: a project
// has tens of components, not thousands, so the jumps the shared default
// makes (15, 25, 50, 100) are bigger than anything here needs.
const PAGE_SIZES = [10, 20, 30, 40, 50];

function cmpStr(a: string, b: string): number {
  return a.localeCompare(b);
}

// ComponentsView lists the project's Jira components and creates, edits and
// deletes them. Writes go straight to Jira; afterwards the list, the local
// test counts and the rest of the profile's data reload.
//
// The layout is the one the Preconditions and Containers views use: a paged
// master list on the left, a detail pane on the right. A component's own
// fields are a handful, so most of that pane is the tests carrying it, which
// is the question somebody opening this view is usually asking.
export function ComponentsView({ onChanged }: { onChanged: () => void }) {
  const { activeId: profileId } = useProfile();
  const qc = useQueryClient();
  const list = useProjectComponents(profileId);
  const counts = useComponentCounts(profileId);
  const [dialog, setDialog] = useState<Dialog>({ kind: "none" });
  const [error, setError] = useState("");

  const [filter, setFilter] = useState("");
  const [sortField, setSortField] = useState("name");
  const [sortDesc, setSortDesc] = useState(false);
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState(10);
  const [selectedId, setSelectedId] = useState("");

  const [testsPage, setTestsPage] = useState(0);
  const [testsPageSize, setTestsPageSize] = useState(10);

  const components = useMemo(() => list.data ?? [], [list.data]);
  const countOf = (c: ProjectComponent) => counts.data?.get(c.name) ?? 0;

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    const base = q
      ? components.filter(
          (c) =>
            c.name.toLowerCase().includes(q) ||
            c.description.toLowerCase().includes(q) ||
            (c.leadDisplayName || c.leadName).toLowerCase().includes(q),
        )
      : components;
    const dir = sortDesc ? -1 : 1;
    return [...base].sort((a, b) => {
      switch (sortField) {
        case "tests":
          return (
            dir *
            ((counts.data?.get(a.name) ?? 0) - (counts.data?.get(b.name) ?? 0))
          );
        case "lead":
          return (
            dir *
            cmpStr(
              a.leadDisplayName || a.leadName,
              b.leadDisplayName || b.leadName,
            )
          );
        default:
          return dir * cmpStr(a.name, b.name);
      }
    });
  }, [components, filter, sortField, sortDesc, counts.data]);

  useEffect(() => {
    setPage(0);
  }, [filter, sortField, sortDesc]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / pageSize));
  const safePage = Math.min(page, totalPages - 1);
  const pageItems = filtered.slice(
    safePage * pageSize,
    (safePage + 1) * pageSize,
  );

  // Keep a selection only while it still exists; otherwise fall to the first
  // row on the page, so the detail pane is never empty beside a full list.
  const selected = components.find((c) => c.id === selectedId) ?? null;
  useEffect(() => {
    if (components.length === 0) {
      setSelectedId("");
      return;
    }
    if (!components.some((c) => c.id === selectedId)) {
      setSelectedId(filtered[0]?.id ?? "");
    }
  }, [components, filtered, selectedId]);

  useEffect(() => {
    setTestsPage(0);
  }, [selectedId]);

  const tests = useComponentTests(
    profileId,
    selected?.name ?? "",
    testsPage,
    testsPageSize,
  );

  async function reload() {
    await qc.invalidateQueries({ queryKey: keys.components(profileId) });
    onChanged();
  }

  async function write(run: () => Promise<unknown>) {
    try {
      await run();
      setDialog({ kind: "none" });
      setError("");
    } finally {
      await reload();
    }
  }

  async function openDelete(c: ProjectComponent) {
    setError("");
    try {
      const issueCount = await ComponentIssueCount(profileId, c.id);
      setDialog({ kind: "delete", component: c, issueCount });
    } catch (e) {
      setError(errMsg(e));
    }
  }

  const others = (c: ProjectComponent) =>
    components.filter((o) => o.id !== c.id);

  return (
    <div className="components-page">
      {/* Every other write in XTM is queued and reaches Jira on Commit. These
          do not, and nothing on screen would otherwise say so. */}
      <div className="components-live-note" role="note">
        Changes here are written to Jira straight away. Unlike the rest of XTM
        they are not queued, and cannot be reviewed or discarded before Commit.
      </div>

      {(error || list.error) && (
        <div className="error-text">{error || errMsg(list.error)}</div>
      )}

      <div
        className={`components-view${selected ? " components-with-detail" : ""}`}
      >
        <div className="components-list">
          <div className="components-list-head">
            <input
              className="components-search"
              value={filter}
              placeholder="Filter components…"
              aria-label="Filter components"
              onChange={(e) => setFilter(e.target.value)}
            />
            <SortControl
              fields={[
                { value: "name", label: "Name" },
                { value: "tests", label: "Tests" },
                { value: "lead", label: "Lead" },
              ]}
              field={sortField}
              desc={sortDesc}
              onChange={(f, d) => {
                setSortField(f);
                setSortDesc(d);
              }}
            />
            <button
              className="btn btn-primary components-new"
              onClick={() => setDialog({ kind: "new" })}
            >
              New component
            </button>
          </div>

          {list.isLoading ? (
            <p className="muted">Loading components…</p>
          ) : components.length === 0 ? (
            <p className="muted">This project has no components yet.</p>
          ) : filtered.length === 0 ? (
            <p className="muted">No component matches that filter.</p>
          ) : (
            <>
              <ul className="components-items">
                {pageItems.map((c) => (
                  <li key={c.id}>
                    <button
                      className={`components-item${c.id === selectedId ? " components-item-active" : ""}`}
                      aria-current={c.id === selectedId ? "true" : undefined}
                      onClick={() => setSelectedId(c.id)}
                    >
                      <div className="components-item-top">
                        <span className="components-item-name">{c.name}</span>
                        <span className="components-item-count">
                          {countOf(c)} test{countOf(c) === 1 ? "" : "s"}
                        </span>
                      </div>
                      <div className="components-item-meta muted">
                        {c.description || "No description"}
                      </div>
                    </button>
                  </li>
                ))}
              </ul>
              <Pager
                compact
                page={safePage}
                pageSize={pageSize}
                total={filtered.length}
                pageSizeOptions={PAGE_SIZES}
                onPage={setPage}
                onPageSize={(n) => {
                  setPageSize(n);
                  setPage(0);
                }}
              />
            </>
          )}
        </div>

        {selected && (
          <div className="components-detail">
            <div className="components-detail-head">
              <h3 className="components-detail-name">{selected.name}</h3>
              <div className="components-detail-actions">
                <button
                  className="btn btn-ghost"
                  aria-label={`Edit ${selected.name}`}
                  onClick={() =>
                    setDialog({ kind: "edit", component: selected })
                  }
                >
                  Edit
                </button>
                <button
                  className="btn btn-danger"
                  aria-label={`Delete ${selected.name}`}
                  onClick={() => openDelete(selected)}
                >
                  Delete
                </button>
              </div>
            </div>

            <dl className="components-detail-fields">
              <div>
                <dt>Description</dt>
                <dd>
                  {selected.description || <span className="muted">None</span>}
                </dd>
              </div>
              <div>
                <dt>Lead</dt>
                <dd>
                  {selected.leadDisplayName || selected.leadName || (
                    <span className="muted">Unassigned</span>
                  )}
                </dd>
              </div>
            </dl>

            <h4 className="components-tests-head">
              Tests carrying this component
            </h4>
            {tests.isLoading ? (
              <p className="muted">Loading tests…</p>
            ) : tests.error ? (
              <p className="error-text">{errMsg(tests.error)}</p>
            ) : (tests.data?.total ?? 0) === 0 ? (
              <p className="muted">No synced test carries this component.</p>
            ) : (
              <>
                <div className="components-tests-scroll">
                  <table className="board-table components-tests">
                    <thead>
                      <tr>
                        <th>Key</th>
                        <th>Summary</th>
                        <th>Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(tests.data?.tests ?? []).map((t) => (
                        <tr key={t.key}>
                          <td className="mono">{t.key}</td>
                          <td>{t.summary}</td>
                          <td>{t.status || <span className="muted">None</span>}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <Pager
                  page={testsPage}
                  pageSize={testsPageSize}
                  total={tests.data?.total ?? 0}
                  pageSizeOptions={PAGE_SIZES}
                  onPage={setTestsPage}
                  onPageSize={(n) => {
                    setTestsPageSize(n);
                    setTestsPage(0);
                  }}
                />
              </>
            )}
          </div>
        )}
      </div>

      {dialog.kind === "new" && (
        <ComponentFormModal
          takenNames={components.map((c) => c.name)}
          onSubmit={(input: ComponentInput) =>
            write(() => CreateComponent(profileId, input))
          }
          onCancel={() => setDialog({ kind: "none" })}
        />
      )}
      {dialog.kind === "edit" && (
        <ComponentFormModal
          initial={dialog.component}
          takenNames={others(dialog.component).map((c) => c.name)}
          onSubmit={(input: ComponentInput) =>
            write(() => UpdateComponent(profileId, dialog.component.id, input))
          }
          onCancel={() => setDialog({ kind: "none" })}
        />
      )}
      {dialog.kind === "delete" && (
        <DeleteComponentModal
          component={dialog.component}
          others={others(dialog.component)}
          issueCount={dialog.issueCount}
          onConfirm={(moveTo) =>
            write(() => DeleteComponent(profileId, dialog.component.id, moveTo))
          }
          onCancel={() => setDialog({ kind: "none" })}
        />
      )}
    </div>
  );
}
