import { useEffect, useState } from "react";
import { Modal } from "@agile-suite/core";
import { GetDiagnostics, BrowserOpenURL } from "../api";
import type { Diagnostics } from "../api";
import appIcon from "../assets/images/appicon.png";

const DOCS_URL = "https://docs.atlassian.com/software/jira/docs/api/REST/latest/";

// LogoMark shows the application's own icon (the same image used for the
// window and the executable), so About matches the app's identity everywhere.
function LogoMark() {
  return (
    <img
      className="about-logo"
      src={appIcon}
      width={44}
      height={44}
      alt=""
      aria-hidden="true"
    />
  );
}

// AboutModal is the Help → About dialog, laid out exactly like XTM's: app
// identity in a dark "scan panel" banner, then a precise technical readout of
// the runtime on a sunken plate, useful to paste into a bug report. TAM's rows
// carry two paths XTM has no equivalent for, the shared profile database and
// the log, because they are the first two things to check when the suite's two
// windows disagree about which profiles exist.
export function AboutModal({ onClose }: { onClose: () => void }) {
  const [diag, setDiag] = useState<Diagnostics | null>(null);

  useEffect(() => {
    GetDiagnostics()
      .then(setDiag)
      .catch(() => {});
  }, []);

  // The third element marks a filesystem path, which the readout truncates
  // from the left so the filename stays visible.
  const rows: Array<[string, string, boolean?]> = [
    ["Version", diag?.version ? `v${diag.version}` : "…"],
    ["Schema", diag ? `v${diag.schemaVersion}` : "…"],
    ["Runtime", diag ? `${diag.goVersion} · ${diag.os}/${diag.arch}` : "…"],
    ["Targets", "Jira DC 8.14+"],
    ["Database", diag?.dbPath || "—", true],
    ["Profiles", diag?.sharedPath || "—", true],
    ["Log", diag?.logPath || "—", true],
  ];

  return (
    <Modal onClose={onClose} className="about-card" label="About Task Activity Manager">
      <button className="about-close" onClick={onClose} title="Close" aria-label="Close">
        ✕
      </button>

      <header className="about-banner">
        <span className="about-banner-grid" aria-hidden="true" />
        <LogoMark />
        <div className="about-id">
          <h2 className="about-name">Task Activity Manager</h2>
          <p className="about-kicker">Agile Backlog · Data Center</p>
        </div>
      </header>

      <div className="about-body">
        <p className="about-tagline">
          Planning and tracking Jira Data Center work, offline. Part of the agile
          suite with Xray Test Manager.
        </p>

        <div className="about-plate">
          <span className="about-plate-label">Build &amp; environment</span>
          <dl className="about-info">
            {rows.map(([k, v, isPath]) => (
              <div className="about-row" key={k}>
                <dt>{k}</dt>
                <dd className={`mono${isPath ? " about-path" : ""}`} title={v}>{v}</dd>
              </div>
            ))}
          </dl>
        </div>

        <div className="about-actions">
          <button className="btn about-link" onClick={() => BrowserOpenURL(DOCS_URL)}>
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
