import { normalizeJiraUrl } from "../profileValidation";
import type { ProfileFormApi } from "./useProfileForm";

// The text fields every profile form has. Labels and placeholders are the
// app's, because the apps word them for their own users; behaviour is shared.

interface FieldProps {
  f: ProfileFormApi;
  label: string;
  placeholder?: string;
}

export function NameField({ f, label, placeholder }: FieldProps) {
  return (
    <label>
      {label}
      <input value={f.name} onChange={(e) => f.setName(e.target.value)} placeholder={placeholder} />
    </label>
  );
}

// UrlField normalises the URL when the field loses focus, and its error is the
// input's description so a screen reader reads it with the field.
export function UrlField({ f, label, placeholder }: FieldProps) {
  return (
    <label>
      {label}
      <input
        value={f.url}
        onChange={(e) => f.setUrl(e.target.value)}
        onBlur={() => f.setUrl(normalizeJiraUrl(f.url))}
        placeholder={placeholder}
        spellCheck={false}
        aria-invalid={f.urlError ? true : undefined}
        aria-describedby={f.urlError ? f.urlErrorId : undefined}
      />
      {f.urlError && <span id={f.urlErrorId} className="field-error">{f.urlError}</span>}
    </label>
  );
}

// KeyField upper-cases a Jira key as it is typed; in "free" key mode (a Kiwi
// product name) it takes the text as it is.
export function KeyField({ f, label, placeholder }: FieldProps) {
  return (
    <label>
      {label}
      <input
        value={f.key}
        onChange={(e) => f.setKey(e.target.value)}
        placeholder={placeholder}
        spellCheck={false}
        aria-invalid={f.keyError ? true : undefined}
        aria-describedby={f.keyError ? f.keyErrorId : undefined}
      />
      {f.keyError && <span id={f.keyErrorId} className="field-error">{f.keyError}</span>}
    </label>
  );
}

export function ScopeField({ f, placeholder }: Omit<FieldProps, "label">) {
  return (
    <label>
      Scope JQL (optional)
      <input value={f.scope} onChange={(e) => f.setScope(e.target.value)} placeholder={placeholder} />
    </label>
  );
}
