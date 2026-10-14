import type { ReactNode } from "react";
import { Modal } from "../Modal";
import type { DiagnosticsInfo, InfoRow } from "./diagnostics";

interface Props {
  appName: string;
  kicker: string;
  tagline: string;
  logoSrc: string;
  // What the app talks to, e.g. "Jira DC 8.14+".
  targets: string;
  docsUrl: string;
  // null until the app's GetDiagnostics answers; the rows read "…" meanwhile.
  diagnostics: DiagnosticsInfo | null;
  // Rows the app adds after the shared ones, e.g. TAM's shared profile store.
  extraRows?: InfoRow[];
  // Buttons the app adds before Documentation, e.g. XTM's tour.
  actions?: ReactNode;
  onOpenUrl: (url: string) => void;
  onClose: () => void;
}

// AboutDialog is Help → About for both apps: the app's identity in a dark
// banner, then a technical readout of the runtime on a sunken plate, which is
// what a bug report wants pasted into it. Each app's AboutModal fetches its own
// diagnostics and passes its words; the layout is this one.
export function AboutDialog({
  appName, kicker, tagline, logoSrc, targets, docsUrl, diagnostics: d,
  extraRows = [], actions, onOpenUrl, onClose,
}: Props) {
  const rows: InfoRow[] = [
    ["Version", d?.version ? `v${d.version}` : "…"],
    ["Schema", d ? `v${d.schemaVersion}` : "…"],
    ["Runtime", d ? `${d.goVersion} · ${d.os}/${d.arch}` : "…"],
    ["Targets", targets],
    ["Database", d?.dbPath || "not known", true],
    ...extraRows,
  ];

  return (
    <Modal onClose={onClose} className="about-card" label={`About ${appName}`}>
      <button className="about-close" onClick={onClose} title="Close" aria-label="Close">
        ✕
      </button>

      <header className="about-banner">
        <span className="about-banner-grid" aria-hidden="true" />
        <img className="about-logo" src={logoSrc} width={44} height={44} alt="" aria-hidden="true" />
        <div className="about-id">
          <h2 className="about-name">{appName}</h2>
          <p className="about-kicker">{kicker}</p>
        </div>
      </header>

      <div className="about-body">
        <p className="about-tagline">{tagline}</p>

        <div className="about-plate">
          <span className="about-plate-label">Build &amp; environment</span>
          <dl className="about-info" role="group" aria-label="Build and environment">
            {rows.map(([k, v, isPath]) => (
              <div className="about-row" key={k}>
                <dt>{k}</dt>
                <dd className={`mono${isPath ? " about-path" : ""}`} title={v}>
                  {v}
                </dd>
              </div>
            ))}
          </dl>
        </div>

        <div className="about-actions">
          {actions}
          <button className="btn about-link" onClick={() => onOpenUrl(docsUrl)}>
            Documentation <span className="about-arrow">↗</span>
          </button>
          <button className="btn btn-primary" onClick={onClose}>
            Close
          </button>
        </div>
      </div>

      <footer className="about-foot">© 2026 Achmad Fienan Rahardianto</footer>
    </Modal>
  );
}
