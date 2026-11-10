import { useContext, useState } from "react";
import { ExportWorkbook } from "../wailsjs/go/main/App";
import { errorText } from "./AddProfileDialog";
import type { Sheet } from "./exportSheets";
import { NoticeContext } from "./StatusBar";

// ExportButton saves what a view shows as an Excel workbook. sheets is called
// on click, so it reads the view's filters as they are then. Where the file
// went, or why it was not written, goes to the status bar's ribbon (#452). A
// cancelled dialog says nothing.
export function ExportButton({ name, sheets }: { name: string; sheets: () => Sheet[] | Promise<Sheet[]> }) {
  const notify = useContext(NoticeContext);
  const [busy, setBusy] = useState(false);
  const save = async () => {
    setBusy(true);
    try {
      const path = await ExportWorkbook(name, await sheets());
      if (path) notify({ text: `Saved to ${path}` });
    } catch (e) {
      notify({ text: `Export failed. ${errorText(e)}`, error: true });
    } finally {
      setBusy(false);
    }
  };
  return (
    <button type="button" className="btn btn-ghost" disabled={busy} onClick={() => void save()}>
      Export to Excel
    </button>
  );
}
