import { useEffect, useState } from "react";
import { AboutDialog } from "@agile-suite/core";
import { GetDiagnostics, BrowserOpenURL } from "../api";
import type { Diagnostics } from "../api";
import appIcon from "../assets/images/appicon.png";

// AboutModal is XTM's Help → About: core's AboutDialog with XTM's words.
//
// onTakeTour, when supplied, adds a button that closes this dialog and replays
// the onboarding tour. About is where people look for help, so the tour is
// offered here as well as in the More menu (RND_P_4TFINT_05-335).
export function AboutModal({ onClose, onTakeTour }: { onClose: () => void; onTakeTour?: () => void }) {
  const [diag, setDiag] = useState<Diagnostics | null>(null);
  useEffect(() => {
    GetDiagnostics()
      .then(setDiag)
      .catch(() => {});
  }, []);
  return (
    <AboutDialog
      appName="Xray Test Manager"
      kicker="Test Repository · Data Center"
      tagline="A fast, local-first desktop tool for managing Xray test cases in Jira Data Center at scale."
      logoSrc={appIcon}
      targets="Jira DC 8.14+ · Xray 8.4.0"
      docsUrl="https://docs.getxray.app/display/XRAY/REST+API"
      diagnostics={diag}
      actions={
        onTakeTour && (
          <button
            className="btn about-link"
            onClick={() => {
              onClose();
              onTakeTour();
            }}
            title="Replay the onboarding walkthrough"
          >
            🧭 Take the tour
          </button>
        )
      }
      onOpenUrl={BrowserOpenURL}
      onClose={onClose}
    />
  );
}
