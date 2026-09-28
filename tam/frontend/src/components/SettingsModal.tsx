import { useEffect, useId, useState } from "react";
import { Modal, errMsg } from "@agile-suite/core";
import {
  ChooseReportExportDirectory,
  GetSettings,
  SetProjectKeyPatternCheck,
  SetReportExportDirectory,
} from "../api";

// SettingsModal holds the preferences that belong to the app rather than to a
// profile and that need more than a menu tick: today that is the folder a
// sprint report's save dialog starts in, and whether a profile's project key
// is checked against Jira's usual shape. Theme and the navigation rail stay
// in the native menu, where a checkbox says everything they need to.
//
// The folder is checked in Go, not here: the path may be typed, pasted or
// picked, and only the machine the app runs on can say whether it is a folder.
// A refusal keeps the dialog open with the reason on it.
export function SettingsModal({ onClose }: { onClose: () => void }) {
  const titleId = useId();
  const fieldId = useId();
  const checkId = useId();
  const [dir, setDir] = useState("");
  // The project key pattern check, on for anyone who has never set it: an
  // absent setting is not a stored "off", so an install that predates the
  // switch keeps the check it has always had.
  const [checkKey, setCheckKey] = useState(true);
  const [loaded, setLoaded] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let live = true;
    GetSettings()
      .then((s) => {
        if (!live) return;
        setDir(s.reportExportDir ?? "");
        setCheckKey(s.checkProjectKeyPattern !== false);
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
      await SetProjectKeyPatternCheck(checkKey);
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
      <label className="check-row" htmlFor={checkId}>
        <input
          id={checkId}
          type="checkbox"
          checked={checkKey}
          disabled={!loaded || saving}
          onChange={(e) => setCheckKey(e.target.checked)}
        />
        Check the project key against Jira's usual shape
      </label>
      <span className="field-hint">
        A profile's project key has to start with a letter and use only letters, digits and underscores,
        which is the shape Jira documents and catches most typos. Turn it off if your Jira accepts keys
        in another shape: the key is then taken as typed, and Jira accepts or refuses it.
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
