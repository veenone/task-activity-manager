import { DiagnosticsDialog } from "@agile-suite/core";
import { ExportDiagnostics, GetDiagnostics, ReadLog } from "../api";

// The log's last 500 lines: enough for the sync that just failed, short enough
// to read in the dialog (FR-12.4).
const readRecentLog = () => ReadLog(500);

// DiagnosticsModal is XTM's diagnostics (FR-12.4): core's dialog with XTM's
// bindings, plus the profile count XTM has always shown.
export function DiagnosticsModal({ onClose }: { onClose: () => void }) {
  return (
    <DiagnosticsDialog
      appName="XTM"
      getDiagnostics={GetDiagnostics}
      readLog={readRecentLog}
      exportDiagnostics={ExportDiagnostics}
      extraRows={(d) => [["Profiles", String(d.profileCount)]]}
      onClose={onClose}
    />
  );
}
