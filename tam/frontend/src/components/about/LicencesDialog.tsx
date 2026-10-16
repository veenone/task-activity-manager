import { useId, useMemo, useState } from "react";
import { Modal } from "@agile-suite/core";

export interface NoticePackage {
  name: string;
  version: string;
  // SPDX identifier, or "see text" when the generator could not tell.
  license: string;
  kind: "go" | "npm";
  // Key into Notices.texts; empty when the package ships no licence file.
  text: string;
}

// Notices is the shape scripts/third-party-notices.mjs writes to
// assets/third-party.json. Identical licence texts are stored once in `texts`.
export interface Notices {
  license: string;
  text: string;
  packages: NoticePackage[];
  texts: Record<string, string>;
}

const KIND_LABEL: Record<NoticePackage["kind"], string> = { go: "Go", npm: "npm" };

function packageKey(p: NoticePackage) {
  return `${p.kind}:${p.name}@${p.version}`;
}

// LicencesDialog is Help → About → Licences: TAM's own licence, then every Go
// module and npm package bundled into the binary with its licence. It opens on
// top of About, so closing it lands back there.
export function LicencesDialog({ notices, onClose }: { notices: Notices; onClose: () => void }) {
  const [filter, setFilter] = useState("");
  const [open, setOpen] = useState<Set<string>>(new Set());
  const idPrefix = useId();

  const shown = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return notices.packages;
    return notices.packages.filter((p) =>
      [p.name, p.license, KIND_LABEL[p.kind]].some((field) => field.toLowerCase().includes(q)),
    );
  }, [filter, notices.packages]);

  function toggle(key: string) {
    setOpen((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  return (
    <Modal onClose={onClose} className="modal pending-modal" labelledBy="licences-title">
      <div className="pending-head">
        <h2 id="licences-title">Licences</h2>
        <button type="button" className="btn btn-ghost" onClick={onClose} aria-label="Close">
          ✕
        </button>
      </div>

      <div className="bulk-body">
        <details>
          <summary>Task Activity Manager is licensed under the GNU GPL v3. Show the licence text.</summary>
          <pre className="diag-log">{notices.texts[notices.text]}</pre>
        </details>

        <input
          type="search"
          className="detail-input"
          aria-label="Filter packages"
          placeholder="Filter by package, licence or kind"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />

        <div className="pending-table-wrap">
          <table className="pending-table">
            <caption className="sr-only">
              Third-party packages bundled with Task Activity Manager, {shown.length} of {notices.packages.length}
            </caption>
            <thead>
              <tr>
                <th scope="col">Package</th>
                <th scope="col">Version</th>
                <th scope="col">Licence</th>
                <th scope="col">Kind</th>
              </tr>
            </thead>
            <tbody>
              {shown.length === 0 && (
                <tr>
                  <td colSpan={4} className="muted">
                    No package matches the filter.
                  </td>
                </tr>
              )}
              {shown.map((p) => {
                const key = packageKey(p);
                const expanded = open.has(key);
                const textId = `${idPrefix}-${key}`;
                return [
                  <tr key={key}>
                    <th scope="row">
                      <button
                        type="button"
                        className="btn btn-ghost"
                        aria-expanded={expanded}
                        aria-controls={expanded ? textId : undefined}
                        onClick={() => toggle(key)}
                      >
                        {p.name}
                      </button>
                    </th>
                    <td className="mono">{p.version}</td>
                    <td>{p.license}</td>
                    <td>{KIND_LABEL[p.kind]}</td>
                  </tr>,
                  expanded && (
                    <tr key={`${key}-text`} id={textId}>
                      <td colSpan={4}>
                        <pre className="diag-log">
                          {notices.texts[p.text] ??
                            `This package ships no licence file. Its declared licence is ${p.license}.`}
                        </pre>
                      </td>
                    </tr>
                  ),
                ];
              })}
            </tbody>
          </table>
        </div>
      </div>

      <div className="pending-actions">
        <span className="muted small">
          {notices.packages.length} third-party packages. Select a package to read its licence.
        </span>
        <span className="pending-footer-buttons">
          <button type="button" className="btn btn-primary" onClick={onClose}>
            Close
          </button>
        </span>
      </div>
    </Modal>
  );
}
