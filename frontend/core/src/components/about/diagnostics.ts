// DiagnosticsInfo is the runtime readout both apps' GetDiagnostics binding
// returns. TAM's also names its shared profile store; an app passes that, or
// anything else of its own, as an extra row.
export interface DiagnosticsInfo {
  version: string;
  dbPath: string;
  logPath: string;
  os: string;
  arch: string;
  goVersion: string;
  schemaVersion: number;
  profileCount: number;
  startupError: string;
}

// InfoRow is one line of a readout: a label, its value, and whether the value
// is a filesystem path, which is truncated from the left so the file name
// stays visible.
export type InfoRow = [label: string, value: string, isPath?: boolean];
