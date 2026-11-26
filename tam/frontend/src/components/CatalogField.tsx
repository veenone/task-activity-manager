import { MultiSelect } from "@agile-suite/core";
import type { CatalogField as Field } from "../api";
import { PERM_RESOLVE_ISSUES } from "../api/permissions";
import { useComponents, useVersions } from "../queries/catalog";
import { useDenial, usePermissions } from "../queries/permissions";
import { catalogOptions, idsText, parseIds } from "../lib/catalog";

interface Props {
  profileId: string;
  // issueKey and project are the issue's, and both "" in New issue, where
  // the draft belongs to the profile's own project.
  issueKey: string;
  project: string;
  field: Field;
  id: string;
  label: string;
  // value is the journal's form, a comma list of catalog ids.
  value: string;
  onChange: (value: string) => void;
  disabled: boolean;
  describedBy?: string;
}

// CatalogField is the issue panel's Components or Fix versions control: the
// shared multi-select over the catalog the last sync cached, drafts
// included.
//
// The catalog is the profile's project's, so an issue from another project
// shows its field read-only: its ids name that project's releases, which
// nothing here can list. A fix version also needs Resolve Issues, which Jira
// checks apart from Edit Issues, so it is turned off with the reason when
// the token lacks it.
export function CatalogField({ profileId, issueKey, project, field, id, label, value, onChange, disabled, describedBy }: Props) {
  const versions = useVersions(profileId);
  const components = useComponents(profileId);
  const home = usePermissions(profileId).data?.project ?? "";
  const resolveDenied = useDenial(profileId)(PERM_RESOLVE_ISSUES, issueKey || undefined);
  const rows = (field === "fixVersions" ? versions.data : components.data) ?? [];
  const selected = parseIds(value);
  const foreign = home !== "" && project !== "" && project !== home;
  const denied = field === "fixVersions" ? resolveDenied : "";
  const reason = foreign ? `${project} is not this profile's project, so its ${label.toLowerCase()} are set in Jira.` : denied;
  const noteId = `${id}-why`;
  return (
    <>
      <MultiSelect
        id={id}
        label={label}
        options={catalogOptions(rows, selected)}
        selected={selected.map(String)}
        onChange={(next) => onChange(idsText(next.map(Number)))}
        allLabel="None"
        describedBy={[describedBy, reason ? noteId : ""].filter(Boolean).join(" ") || undefined}
        disabled={disabled || foreign || !!denied}
      />
      {reason && <p className="muted small" id={noteId}>{reason}</p>}
    </>
  );
}
