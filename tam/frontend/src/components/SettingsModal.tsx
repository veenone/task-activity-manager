import { useEffect, useId, useState } from "react";
import { Modal, errMsg } from "@agile-suite/core";
import { ChooseReportExportDirectory, GetSettings, SetReportExportDirectory } from "../api";

// SettingsModal holds the preferences that belong to the app rather than to a
// profile and that need more than a menu tick: today that is the folder a
// sprint report's save dialog starts in. Theme and the navigation rail stay
// in the native menu, where a checkbox says everything they need to.
//
// The folder is checked in Go, not here: the path may be typed, pasted or
// picked, and only the machine the app runs on can say whether it is a folder.
// A refusal keeps the dialog open with the reason on it.
export function SettingsModal({ onClose }: { onClose: () => void }) {
  const titleId = useId();
  const fieldId = useId();
  const [dir, setDir] = useState("");
  const [loaded, setLoaded] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let live = true;
    GetSettings()
      .then((s) => {
        if (!live) return;
        setDir(s.reportExportDir ?? "");
        setLoaded(true);
      })
      .catch((e) => live && setError(errMsg(e)));
    return () => {
      live = false;
    };
  }, []);

  async function browse() {
    try {
      const picked = await ChooseReportExportDirectory();
      // "" is the picker closed without choosing, which changes nothing.
      if (picked) setDir(picked);
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function save() {
    setSaving(true);
    setError("");
    try {
      await SetReportExportDirectory(dir.trim());
      onClose();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal onClose={onClose} className="modal" labelledBy={titleId}>
      <h2 id={titleId}>Settings</h2>
      <label htmlFor={fieldId}>Report export folder</label>
      <input
        id={fieldId}
        className="detail-input"
        value={dir}
        disabled={!loaded || saving}
        spellCheck={false}
        placeholder="The app data folder"
        onChange={(e) => setDir(e.target.value)}
      />
      <span className="field-hint">
        Where the Save dialog opens when you export a sprint report as a spreadsheet or a deck.
        Leave it blank for the app data folder, and you can still save anywhere from the dialog itself.
      </span>
      {error && <p className="error-text" role="alert">{error}</p>}
      <div className="form-actions form-actions-end">
        <button type="button" className="btn" onClick={() => void browse()} disabled={saving}>Browse...</button>
        <button type="button" className="btn" onClick={onClose} disabled={saving}>Cancel</button>
        <button type="button" className="btn btn-primary" onClick={() => void save()} disabled={!loaded || saving}>
          {saving ? "Saving..." : "Save"}
        </button>
      </div>
    </Modal>
  );
}
