import { useEffect, useState } from "react";
import { AboutDialog } from "@agile-suite/core";
import { GetDiagnostics, BrowserOpenURL } from "../api";
import type { Diagnostics } from "../api";
import appIcon from "../assets/images/appicon.png";

// AboutModal is TAM's Help → About: core's AboutDialog with TAM's words. Its
// readout adds two paths XTM has no equivalent for, the shared profile store
// and the log, because they are the first two things to check when the
// suite's two windows disagree about which profiles exist.
export function AboutModal({ onClose }: { onClose: () => void }) {
  const [diag, setDiag] = useState<Diagnostics | null>(null);
  useEffect(() => {
    GetDiagnostics()
      .then(setDiag)
      .catch(() => {});
  }, []);
  return (
    <AboutDialog
      appName="Task Activity Manager"
      kicker="Agile Backlog · Data Center"
      tagline="Planning and tracking Jira Data Center work, offline. Part of the agile suite with Xray Test Manager."
      logoSrc={appIcon}
      targets="Jira DC 8.14+"
      docsUrl="https://docs.atlassian.com/software/jira/docs/api/REST/latest/"
      diagnostics={diag}
      extraRows={[
        ["Profiles", diag?.sharedPath || "not known", true],
        ["Log", diag?.logPath || "not known", true],
      ]}
      onOpenUrl={BrowserOpenURL}
      onClose={onClose}
    />
  );
}
