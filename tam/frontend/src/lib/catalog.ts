import type { MultiOption } from "@agile-suite/core";
import type { Component, Version } from "../api";

// The journal holds an issue's components and fix versions as a comma list
// of catalog ids, the same form backend.FormatIDs writes. parseIds reads it
// back with blanks and repeats dropped and the order kept.
export function parseIds(text: string): number[] {
  const out: number[] = [];
  for (const part of text.split(",")) {
    const id = Number(part.trim());
    if (part.trim() !== "" && Number.isInteger(id) && !out.includes(id)) out.push(id);
  }
  return out;
}

export function idsText(ids: number[]): string {
  return ids.join(", ");
}

function isVersion(row: Version | Component): row is Version {
  return "released" in row;
}

// versionState is the word a release's option carries beside its name.
function versionState(v: Version): string {
  if (v.draft) return "draft";
  if (v.archived) return "archived";
  if (v.released) return "released";
  return "";
}

// catalogOptions are the picker's options, in the order the store keeps
// them: releases by release date then name, components by name. A released
// or archived release cannot be newly picked, so it is disabled unless the
// issue already has it, where it stays clearable.
export function catalogOptions(rows: (Version | Component)[], selected: number[]): MultiOption[] {
  return rows.map((row) => {
    const state = isVersion(row) ? versionState(row) : "";
    const closed = isVersion(row) && (row.released || row.archived);
    return {
      value: String(row.id),
      label: state ? `${row.name} (${state})` : row.name,
      disabled: closed && !selected.includes(row.id),
    };
  });
}

// catalogNames is a journal value read out as names, for the Pending
// changes dialog. An id the cache no longer holds is shown as its number
// rather than dropped.
export function catalogNames(rows: (Version | Component)[], text: string): string {
  return parseIds(text)
    .map((id) => rows.find((r) => r.id === id)?.name ?? String(id))
    .join(", ");
}
