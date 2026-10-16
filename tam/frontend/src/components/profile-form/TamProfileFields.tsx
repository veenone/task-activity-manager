import { useId } from "react";
import type { TamProfileExtras } from "./useTamProfileExtras";

// TamProfileFields are the two settings only TAM has: the issue type it syncs
// as a requirement, and how long a cached issue detail stays fresh.
export function TamProfileFields({ x }: { x: TamProfileExtras }) {
  const minutesErrorId = useId();
  return (
    <>
      <label>
        Requirement issue type (optional)
        <input
          value={x.requirementType}
          onChange={(e) => x.setRequirementType(e.target.value)}
          placeholder="Requirement"
          spellCheck={false}
        />
        <span className="field-hint">
          The Jira issue type TAM syncs as a requirement. Leave it blank for
          "Requirement". Changing it resets the sync cursor, so the next sync
          pulls everything again; run a Full sync to drop rows of the old type.
        </span>
      </label>
      <label>
        Issue detail freshness (minutes)
        <input
          value={x.detailMinutes}
          onChange={(e) => x.setDetailMinutes(e.target.value)}
          placeholder="10"
          inputMode="numeric"
          spellCheck={false}
          aria-invalid={x.minutesError ? true : undefined}
          aria-describedby={x.minutesError ? minutesErrorId : undefined}
        />
        {x.minutesError && <span id={minutesErrorId} className="field-error">{x.minutesError}</span>}
        <span className="field-hint">
          How long a cached issue detail is shown before TAM asks Jira for it
          again. Leave it blank for 10 minutes. 0 keeps issue details cached
          until you press Refresh, so TAM can be used with no Jira connection.
        </span>
      </label>
    </>
  );
}
