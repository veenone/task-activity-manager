import type { ProfileFormState } from "./useProfileFormState";

// TlsSettings is the CA certificate and the untrusted-TLS switch for this
// connection.
export function TlsSettings({ s }: { s: ProfileFormState }) {
  const { caCert, setCaCert, allowUntrustedTLS, setAllowUntrustedTLS } = s;
  return (
    <>
      <details className="profile-form-advanced">
        <summary>Advanced: TLS / certificate settings (this connection)</summary>
        <label>
          CA certificate (PEM, optional)
          <textarea
            value={caCert}
            onChange={(e) => setCaCert(e.target.value)}
            placeholder={"-----BEGIN CERTIFICATE-----\n…\n-----END CERTIFICATE-----"}
            rows={5}
            spellCheck={false}
          />
          <span className="field-hint">
            Paste a PEM-encoded CA certificate to trust when connecting to this Jira
            instance. You'll need this if the server uses a private or internal CA
            that isn't in your system's trust store (for example, a corporate CA on
            macOS).
          </span>
        </label>
        <label className="profile-form-checkbox">
          <input
            type="checkbox"
            checked={allowUntrustedTLS}
            onChange={(e) => setAllowUntrustedTLS(e.target.checked)}
          />
          Allow untrusted certificate (skip TLS verification)
          <span className="field-hint field-hint-warn">
            Turns off all TLS certificate checks. Only use this for trusted
            internal servers when no CA certificate is available. This is insecure
            and should not be used in production.
          </span>
        </label>
      </details>
    </>
  );
}
