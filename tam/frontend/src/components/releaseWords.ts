import { ENTITY_VERSION_ARCHIVE, ENTITY_VERSION_CREATE, ENTITY_VERSION_DELETE, ENTITY_VERSION_EDIT, ENTITY_VERSION_RELEASE } from "../api";
import type { PendingChange, VersionArchive, VersionDelete, VersionFields, VersionRelease } from "../api";

// The Pending changes dialog's words for release changes (#605), beside
// pendingWords.ts, which holds the sprint and board ones.

function parse<T>(text: string): T | null {
  try {
    return JSON.parse(text) as T;
  } catch {
    return null;
  }
}

// releaseChangeName is the release's name: an edit's before value, the name
// every other change carries.
export function releaseChangeName(row: PendingChange): string {
  const from = row.entityType === ENTITY_VERSION_EDIT ? row.beforeVal : row.afterVal;
  return parse<{ name?: string }>(from)?.name || `release ${row.entityKey}`;
}

// releaseChangeLine reads one release change in words.
export function releaseChangeLine(row: PendingChange, project: string): string {
  const name = releaseChangeName(row);
  switch (row.entityType) {
    case ENTITY_VERSION_CREATE: {
      const d = parse<VersionFields>(row.afterVal);
      if (!d) return "A draft release that could not be read. Discard it and draft it again.";
      const bits = [`New release in ${project}`];
      if (d.startDate) bits.push(`start ${d.startDate}`);
      if (d.releaseDate) bits.push(`release ${d.releaseDate}`);
      return bits.join(", ");
    }
    case ENTITY_VERSION_RELEASE: {
      const r = parse<VersionRelease>(row.afterVal);
      const on = r?.releaseDate ? ` on ${r.releaseDate}` : "";
      const rest = r?.moveToName ? `unfinished issues Jira still finds go to ${r.moveToName}` : "no release named for unfinished issues Jira still finds";
      return `Release ${name}${on}, ${rest}`;
    }
    case ENTITY_VERSION_ARCHIVE:
      return parse<VersionArchive>(row.afterVal)?.archived === false ? `Unarchive ${name}` : `Archive ${name}`;
    case ENTITY_VERSION_DELETE: {
      const d = parse<VersionDelete>(row.afterVal);
      return d?.moveToName ? `Delete ${name}, its issues move to ${d.moveToName}` : `Delete ${name}`;
    }
  }
  const was = parse<VersionFields>(row.beforeVal);
  const now = parse<VersionFields>(row.afterVal);
  if (!was || !now) return `Edit release ${name}: the change could not be read. Discard it and edit the release again.`;
  const changed: string[] = [];
  if (now.name !== was.name) changed.push(`name to ${now.name}`);
  if (now.description !== was.description) changed.push(now.description ? "description changed" : "description removed");
  if (now.startDate !== was.startDate) changed.push(now.startDate ? `start date to ${now.startDate}` : "start date removed");
  if (now.releaseDate !== was.releaseDate) changed.push(now.releaseDate ? `release date to ${now.releaseDate}` : "release date removed");
  return `Edit release ${name}: ${changed.length ? changed.join(", ") : "nothing changed"}`;
}

// releaseChangeNote says what Commit does with a release change.
export function releaseChangeNote(row: PendingChange): string {
  switch (row.entityType) {
    case ENTITY_VERSION_CREATE: return "Commit creates it in Jira before the issue changes that name it.";
    case ENTITY_VERSION_EDIT: return "Saved locally. Commit sends the change to Jira.";
    case ENTITY_VERSION_RELEASE: return "Commit releases it after the issue changes it asked for have reached Jira.";
    case ENTITY_VERSION_ARCHIVE: return "Commit sends this to Jira.";
  }
  return "Commit deletes it in Jira. The issues themselves are not deleted.";
}
