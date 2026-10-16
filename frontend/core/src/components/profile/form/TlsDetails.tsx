import type { ReactNode } from "react";
import type { ProfileFormApi } from "./useProfileForm";

// TlsDetails is the collapsed CA certificate and untrusted-TLS section. The
// summary and the certificate hint are the app's words.
export function TlsDetails({ f, summary, hint }: { f: ProfileFormApi; summary: string; hint: ReactNode }) {
  return (
    <details className="profile-form-advanced">
      <summary>{summary}</summary>
      <label>
        CA certificate (PEM, optional)
        <textarea
          value={f.caCert}
          onChange={(e) => f.setCaCert(e.target.value)}
          placeholder={"-----BEGIN CERTIFICATE-----\n…\n-----END CERTIFICATE-----"}
          rows={5}
          spellCheck={false}
        />
        <span className="field-hint">{hint}</span>
      </label>
      <label className="profile-form-checkbox">
        <input type="checkbox" checked={f.allowTls} onChange={(e) => f.setAllowTls(e.target.checked)} />
        Allow untrusted certificate (skip TLS verification)
        <span className="field-hint field-hint-warn">
          Turns off all TLS certificate checks. Only use this for trusted
          internal servers when no CA certificate is available. This is
          insecure and should not be used in production.
        </span>
      </label>
    </details>
  );
}
