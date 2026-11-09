import { useEffect, useState } from "react";
import { AboutDialog } from "@agile-suite/core";
import { GetDiagnostics } from "../wailsjs/go/main/App";
import type { main } from "../wailsjs/go/models";
import { BrowserOpenURL } from "../wailsjs/runtime/runtime";
import appIcon from "./assets/appicon.png";

// AboutModal is PIE's Help > About: core's AboutDialog, the one TAM and XTM
// use, with PIE's words and its GetDiagnostics readout (#450).
export function AboutModal({ onClose }: { onClose: () => void }) {
  const [diag, setDiag] = useState<main.Diagnostics | null>(null);
  useEffect(() => {
    GetDiagnostics().then(setDiag, () => undefined);
  }, []);
  return (
    <AboutDialog
      appName="Project Issue Explorer"
      kicker="Cross-project issues · Data Center"
      tagline="Follows a Jira filter across projects, traces linked issues, and reports cases and cycles. Part of the agile suite with Task Activity Manager and Xray Test Manager."
      logoSrc={appIcon}
      targets="Jira DC 8.14+, Confluence DC"
      docsUrl="https://docs.atlassian.com/software/jira/docs/api/REST/latest/"
      diagnostics={diag}
      extraRows={[
        ["Profiles", diag ? String(diag.profileCount) : "…"],
        ["Licence", "GNU GPL v3"],
      ]}
      onOpenUrl={BrowserOpenURL}
      onClose={onClose}
    />
  );
}
