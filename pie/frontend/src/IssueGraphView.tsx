import { lazy, Suspense, useContext, useMemo, useRef, useState } from "react";
import { ExpandIssue } from "../wailsjs/go/main/App";
import type { query } from "../wailsjs/go/models";
import { BrowserOpenURL } from "../wailsjs/runtime/runtime";
import type { LayoutOptions } from "@antv/g6";
import type { CanvasApi } from "./G6Canvas";
import {
  add,
  back,
  emptyGraph,
  forward,
  type Graph,
  type History,
  looksLikeKey,
  neighbourKeys,
  PER_EXPAND,
  search,
  typeCounts,
  unloaded,
  visible,
  visit,
} from "./graphModel";
import { IssueDetails } from "./IssueDetails";
import { errorText, isDemo } from "./ProfileForm";
import { NoticeContext } from "./StatusBar";

// G6 is loaded with the view, not with the app, so it costs nothing until
// someone opens the issue graph.
const G6Canvas = lazy(() => import("./G6Canvas"));

const OFFLINE =
  "Offline: Jira cannot be reached, so the graph shows the issues stored on this computer. Run, Refresh links and Publish need Jira.";

// busiestIssue is the run's issue with the most links, where the graph starts.
function busiestIssue(issues: query.Issue[]): string | null {
  let best: query.Issue | null = null;
  for (const is of issues)
    if (!best || neighbourKeys(is).length > neighbourKeys(best).length)
      best = is;
  return best?.key ?? null;
}

// IssueGraphView draws issues and their links with G6 (#464): find an issue,
// expand its links, walk them with back and forward, filter by type.
export function IssueGraphView({
  profileId,
  result,
  jiraUrl,
  label,
  onOpen,
}: {
  profileId: string;
  result: query.Result;
  jiraUrl: string;
  label: (project: string) => string;
  onOpen: (key: string) => void;
}) {
  const notify = useContext(NoticeContext);
  const run = useMemo(
    () => new Map(result.issues.map((i) => [i.key, i])),
    [result],
  );
  const [start] = useState(() => busiestIssue(result.issues));
  const [graph, setGraph] = useState<Graph>(() => {
    if (!start) return emptyGraph();
    const first = run.get(start)!;
    const near = neighbourKeys(first)
      .slice(0, PER_EXPAND)
      .flatMap((k) => (run.has(k) ? [run.get(k)!] : []));
    return add(
      emptyGraph(),
      [first, ...near].map((issue) => ({ issue, inRun: true, fetchedAt: "" })),
    ).graph;
  });
  // The issue the graph is laid out around (#469). Choosing a node only
  // selects it; Find and Centre here move the centre.
  const [centre, setCentre] = useState(start);
  const [history, setHistory] = useState<History>(() =>
    start ? { keys: [start], at: 0 } : { keys: [], at: -1 },
  );
  const [hidden, setHidden] = useState<Set<string>>(new Set());
  const [outside, setOutside] = useState(true);
  const [finished, setFinished] = useState(true);
  const [find, setFind] = useState("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const offline = useRef(false);
  // One expansion at a time: each adds to the graph it read when it began,
  // so a second running alongside would drop the first one's issues.
  const inFlight = useRef(false);
  const canvas = useRef<CanvasApi>(null);
  const focus = history.keys[history.at] ?? null;

  // expand puts key and its next neighbours on the canvas: from the run when
  // it holds them all, otherwise from ExpandIssue, which falls back to the
  // stored copies when Jira cannot be reached.
  async function expand(key: string) {
    if (inFlight.current) return false;
    inFlight.current = true;
    try {
      return await expandNow(key);
    } finally {
      inFlight.current = false;
    }
  }

  async function expandNow(key: string) {
    setError("");
    setMessage("");
    const own = graph.nodes.get(key)?.issue ?? run.get(key);
    const want = own
      ? neighbourKeys(own).filter((k) => !graph.nodes.has(k))
      : [];
    let found: { issue: query.Issue; inRun: boolean; fetchedAt: string }[];
    if (own && want.every((k) => run.has(k))) {
      found = [own, ...want.slice(0, PER_EXPAND).map((k) => run.get(k)!)].map(
        (issue) => ({ issue, inRun: run.has(issue.key), fetchedAt: "" }),
      );
    } else {
      setBusy(true);
      try {
        const n = await ExpandIssue(profileId, key);
        if (n.offline !== offline.current) {
          offline.current = n.offline;
          notify(n.offline ? { text: OFFLINE } : null);
        }
        const [first, ...rest] = n.issues;
        const fresh = rest
          .filter((c) => !graph.nodes.has(c.issue.key))
          .slice(0, PER_EXPAND);
        found = [first, ...fresh].map((c) => ({
          issue: c.issue,
          inRun: run.has(c.issue.key),
          fetchedAt: c.fetchedAt,
        }));
        if (n.missing.length > 0)
          setMessage(
            `${n.missing.length} linked issues are not stored on this computer: ${n.missing.slice(0, 5).join(", ")}${n.missing.length > 5 ? "…" : ""}`,
          );
      } catch (e) {
        setError(errorText(e));
        return false;
      } finally {
        setBusy(false);
      }
    }
    const { graph: next, left } = add(graph, found);
    setGraph(next);
    // Counted on the graph as it now is, from the links Jira just sent.
    const more = unloaded(next, key);
    if (left > 0)
      setMessage(
        `The graph holds at most ${next.nodes.size} issues; ${left} more did not fit. Start again from one of them to see further.`,
      );
    else if (more > 0)
      setMessage(`${more} more links to load: choose Expand links again.`);
    return true;
  }

  async function go(key: string) {
    if (!graph.nodes.has(key) && !(await expand(key))) return false;
    setHistory((h) => visit(h, key));
    return true;
  }

  // Rings of neighbours around the centre, sized to the boxes G6Canvas draws.
  const layout = useMemo<LayoutOptions>(
    () => ({
      type: "radial",
      focusNode: centre ?? undefined,
      unitRadius: 220,
      linkDistance: 220,
      preventOverlap: true,
      nodeSize: 150,
      nodeSpacing: 20,
    }),
    [centre],
  );

  // Kept from render to render, so typing in Find does not redraw the graph.
  const { nodes, edges } = useMemo(() => {
    const shown = visible(
      graph,
      { hiddenTypes: hidden, outside, finished },
      focus,
    );
    return {
      nodes: shown.nodes.map((n) => {
        const left = unloaded(graph, n.key);
        return {
          id: n.key,
          label: n.key,
          sub:
            n.issue.summary.length > 22
              ? `${n.issue.summary.slice(0, 21)}…`
              : n.issue.summary,
          status: n.issue.statusCategory,
          dashed: !n.inRun,
          badge: left > 0 ? `+${left}` : undefined,
        };
      }),
      edges: shown.edges.map((e) => ({
        id: e.id,
        source: e.source,
        target: e.target,
        type: e.type,
        directed: e.directed,
      })),
    };
  }, [graph, hidden, outside, finished, focus]);
  const matches = search(graph, find);
  const chosen = focus ? graph.nodes.get(focus) : undefined;

  if (!focus) return <p className="muted">No issues in this run to draw.</p>;
  return (
    <div className="pie-graph">
      <section className="pie-graph-side" aria-label="Issue graph options">
        <h4>Find an issue</h4>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            const key =
              matches[0]?.key ??
              (looksLikeKey(find) ? find.trim().toUpperCase() : "");
            if (key)
              void go(key).then((ok) => {
                setFind("");
                if (ok) setCentre(key);
              });
          }}
        >
          <input
            type="search"
            className="detail-input"
            aria-label="Find an issue"
            placeholder="Key or summary, e.g. ABC-120"
            value={find}
            onChange={(e) => setFind(e.target.value)}
          />
        </form>
        {matches.length > 0 && (
          <ul className="pie-graph-found" aria-label="Issues found">
            {matches.map((n) => (
              <li key={n.key}>
                <button
                  type="button"
                  className="pie-map-key"
                  onClick={() => void go(n.key).then(() => setFind(""))}
                >
                  <span className="mono">{n.key}</span> {n.issue.summary}
                </button>
              </li>
            ))}
          </ul>
        )}
        {find && matches.length === 0 && looksLikeKey(find) && (
          <p className="muted">Press Enter to fetch it.</p>
        )}
        <h4>Relationships</h4>
        {typeCounts(graph).map(([type, n]) => (
          <label key={type} className="pie-check">
            <input
              type="checkbox"
              checked={!hidden.has(type)}
              onChange={(e) => {
                const next = new Set(hidden);
                if (e.target.checked) next.delete(type);
                else next.add(type);
                setHidden(next);
              }}
            />
            {type}
            <span className="muted">{n}</span>
          </label>
        ))}
        <h4>Show</h4>
        <label className="pie-check">
          <input
            type="checkbox"
            checked={outside}
            onChange={(e) => setOutside(e.target.checked)}
          />
          Issues outside the run
        </label>
        <label className="pie-check">
          <input
            type="checkbox"
            checked={finished}
            onChange={(e) => setFinished(e.target.checked)}
          />
          Finished issues
        </label>
      </section>
      <div className="pie-graph-main">
        <div className="pie-filters">
          <button
            type="button"
            className="btn"
            disabled={history.at <= 0}
            onClick={() => setHistory(back)}
          >
            ‹ Back
          </button>
          <button
            type="button"
            className="btn"
            disabled={history.at >= history.keys.length - 1}
            onClick={() => setHistory(forward)}
          >
            Forward ›
          </button>
          <nav aria-label="Issues visited" className="pie-graph-crumbs">
            {history.keys
              .slice(0, history.at + 1)
              .slice(-5)
              .map((k, i, all) => (
                <span key={`${k}-${i}`}>
                  {i > 0 && " › "}
                  {i === all.length - 1 ? (
                    <b className="mono">{k}</b>
                  ) : (
                    <span className="mono">{k}</span>
                  )}
                </span>
              ))}
          </nav>
          <button
            type="button"
            className="btn"
            onClick={() => canvas.current?.fit()}
          >
            Fit
          </button>
          <button
            type="button"
            className="btn"
            onClick={() => canvas.current?.relayout()}
          >
            Re-layout
          </button>
          <span className="muted" role="status">
            {busy
              ? "Fetching links..."
              : `${graph.nodes.size} issues, ${graph.edges.size} links`}
          </span>
        </div>
        {error && (
          <p role="alert" className="error-text">
            {error}
          </p>
        )}
        {message && <p className="muted">{message}</p>}
        <Suspense fallback={<p className="muted">Loading the graph...</p>}>
          <G6Canvas
            nodes={nodes}
            edges={edges}
            focus={focus}
            layout={layout}
            onSelect={(k) => setHistory((h) => visit(h, k))}
            label={`Issues linked to ${focus}`}
            apiRef={canvas}
          />
        </Suspense>
      </div>
      {chosen && (
        <IssueDetails
          node={chosen}
          label={label}
          unloaded={unloaded(graph, chosen.key)}
          busy={busy}
          centred={chosen.key === centre}
          onCentre={() => setCentre(chosen.key)}
          canOpenInJira={!!jiraUrl && !isDemo(jiraUrl)}
          onExpand={() => void expand(chosen.key)}
          onTrace={() => onOpen(chosen.key)}
          onJira={() =>
            BrowserOpenURL(
              `${jiraUrl.replace(/\/+$/, "")}/browse/${chosen.key}`,
            )
          }
          onPick={(k) => void go(k)}
        />
      )}
    </div>
  );
}
