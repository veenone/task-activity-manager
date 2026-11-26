import { statusCategory } from "@agile-suite/core";
import type { BadgeTone } from "@agile-suite/core";
import { ENTITY_VERSION_ARCHIVE, ENTITY_VERSION_DELETE, ENTITY_VERSION_EDIT, ENTITY_VERSION_RELEASE } from "../api";
import type { Issue, PendingChange, SortColumn, VersionArchive, VersionSummary } from "../api";

// The Releases view's rules, kept out of the components so they can be
// tested on their own.

export type ReleaseFilter = "unreleased" | "released" | "archived" | "all";

export const RELEASE_FILTERS: { id: ReleaseFilter; label: string }[] = [
  { id: "unreleased", label: "Unreleased" },
  { id: "released", label: "Released" },
  { id: "archived", label: "Archived" },
  { id: "all", label: "All" },
];

// isOpen is a release issues can still be added to.
export function isOpen(v: VersionSummary): boolean {
  return !v.released && !v.archived;
}

// filterReleases keeps the releases the filter names. An archived release
// is only under Archived and All, whether or not it was released first.
export function filterReleases(list: VersionSummary[], f: ReleaseFilter): VersionSummary[] {
  switch (f) {
    case "unreleased": return list.filter(isOpen);
    case "released": return list.filter((v) => v.released && !v.archived);
    case "archived": return list.filter((v) => v.archived);
  }
  return list;
}

// releaseStatus is the badge a release wears.
export function releaseStatus(v: VersionSummary): { label: string; tone: BadgeTone } {
  if (v.draft) return { label: "Draft", tone: "draft" };
  if (v.archived) return { label: "Archived", tone: "closed" };
  if (v.released) return { label: "Released", tone: "closed" };
  return { label: "Unreleased", tone: "future" };
}

// releaseDates says what dates the release has, in Jira's own form.
export function releaseDates(v: VersionSummary): string {
  if (v.startDate && v.releaseDate) return `${v.startDate} to ${v.releaseDate}`;
  if (v.releaseDate) return `Release ${v.releaseDate}`;
  if (v.startDate) return `Starts ${v.startDate}`;
  return "No dates";
}

// unfinishedIssues are the issues a release would close over: status
// category not done, or the status name when a row has no category. It is
// the rule backend.Finished applies to the progress counts, so the dialog's
// list and the bar always add up.
export function unfinishedIssues(issues: Issue[]): Issue[] {
  return issues.filter((i) => statusCategory(i.status, i.statusCategory) !== "done");
}

// defaultCarryOver is the release the Release dialog offers first: the next
// open one after the release being closed, in the store's order (release
// date, undated last, then name), else an earlier open one, else none (0).
export function defaultCarryOver(list: VersionSummary[], releasing: number): number {
  const at = list.findIndex((v) => v.id === releasing);
  const after = list.slice(at + 1).find((v) => isOpen(v) && v.id !== releasing);
  if (after) return after.id;
  return list.find((v) => isOpen(v) && v.id !== releasing)?.id ?? 0;
}

// releaseWaiting maps each release id to what Commit will do to it, for the
// chip on its row.
export function releaseWaiting(rows: PendingChange[]): Map<number, string> {
  const out = new Map<number, string>();
  for (const row of rows) {
    const id = Number(row.entityKey);
    switch (row.entityType) {
      case ENTITY_VERSION_RELEASE:
        out.set(id, "Releasing on Commit");
        break;
      case ENTITY_VERSION_DELETE:
        out.set(id, "Deleting on Commit");
        break;
      case ENTITY_VERSION_EDIT:
        if (!out.has(id)) out.set(id, "Edited, not committed");
        break;
      case ENTITY_VERSION_ARCHIVE: {
        let archived = true;
        try {
          archived = (JSON.parse(row.afterVal) as VersionArchive).archived;
        } catch {
          // An unreadable row still archives on the Go side's terms.
        }
        out.set(id, archived ? "Archiving on Commit" : "Unarchiving on Commit");
        break;
      }
    }
  }
  return out;
}

// sortIssues is the release's issue table sorted by the clicked column.
// The release's list is small and read whole, so it sorts here rather than
// in the store; "" keeps the store's rank order.
export function sortIssues(issues: Issue[], column: SortColumn | "", desc: boolean): Issue[] {
  if (!column) return issues;
  const text = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });
  const value = (i: Issue): string | number => {
    switch (column) {
      case "storyPoints": return i.storyPoints ?? -1;
      case "timeSpent": return i.timeSpentSeconds ?? -1;
      case "sprint": return i.sprintName;
      default: return String(i[column] ?? "");
    }
  };
  const out = [...issues].sort((a, b) => {
    const x = value(a);
    const y = value(b);
    return typeof x === "number" && typeof y === "number" ? x - y : text.compare(String(x), String(y));
  });
  return desc ? out.reverse() : out;
}
