import { useState } from "react";
import { ExportWorkbook } from "../wailsjs/go/main/App";
import { errorText } from "./AddProfileDialog";
import type { Sheet } from "./exportSheets";

// ExportButton saves what a view shows as an Excel workbook. sheets is called
// on click, so it reads the view's filters as they are then. A cancelled
// dialog says nothing.
export function ExportButton({ name, sheets }: { name: string; sheets: () => Sheet[] | Promise<Sheet[]> }) {
  const [saved, setSaved] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const save = async () => {
    setBusy(true);
    setSaved("");
    setError("");
    try {
      const path = await ExportWorkbook(name, await sheets());
      if (path) setSaved(path);
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <span className="pie-export">
      <button type="button" className="btn btn-ghost" disabled={busy} onClick={() => void save()}>
        Export to Excel
      </button>
      {/* Mounted from the start, so a screen reader announces the text when it arrives. */}
      <span role="status" className="muted">
        {saved && `Saved to ${saved}`}
      </span>
      {error && (
        <span role="alert" className="error-text">
          {error}
        </span>
      )}
    </span>
  );
}
