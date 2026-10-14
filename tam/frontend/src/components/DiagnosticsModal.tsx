import { DiagnosticsDialog } from "@agile-suite/core";
import { ExportDiagnostics, GetDiagnostics, ReadLog } from "../api";
import type { Diagnostics } from "../api";

// DiagnosticsModal is TAM's diagnostics: core's dialog with TAM's bindings.
// TAM has two databases, so the shared profile store is a row of its own, and
// its log binding takes no arguments, so the dialog has no say in which file
// it gets.
export function DiagnosticsModal({ onClose }: { onClose: () => void }) {
  return (
    <DiagnosticsDialog
      appName="TAM"
      getDiagnostics={GetDiagnostics}
      readLog={ReadLog}
      exportDiagnostics={ExportDiagnostics}
      extraRows={(d) => [["Shared profiles", (d as Diagnostics).sharedPath || "not known", true]]}
      onClose={onClose}
    />
  );
}
