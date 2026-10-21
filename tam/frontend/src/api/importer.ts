// Importing issues from a spreadsheet.

import * as App from "../../wailsjs/go/main/App";
import { importer } from "../../wailsjs/go/models";

export interface ImportPreview {
  headers: string[];
  rowCount: number;
  sample: string[];
}

export interface ImportMapping {
  key: string;
  type: string;
  summary: string;
  description: string;
  priority: string;
  labels: string;
  assignee: string;
  storyPoints: string;
  parentKey: string;
  sprint: string;
}

export interface ImportRowError {
  row: number;
  message: string;
}

export interface ImportResult {
  rows: number;
  created: string[];
  updated: string[];
  errors: ImportRowError[];
  // sprintCellsIgnored counts the keyed rows whose mapped Sprint cell held a
  // value: a sprint is a board write, not a field, so the cell is read by
  // nothing and the row's other fields land without it.
  sprintCellsIgnored: number;
}

// IMPORT_FIELDS are the fields a column can feed, in dialog order. Key comes
// first because it decides what the whole row does: mapped and filled in, the
// row updates the issue it names instead of creating a new one.
export const IMPORT_FIELDS: { id: keyof ImportMapping; label: string }[] = [
  { id: "key", label: "Issue key (updates)" },
  { id: "type", label: "Type" },
  { id: "summary", label: "Summary" },
  { id: "description", label: "Description" },
  { id: "priority", label: "Priority" },
  { id: "labels", label: "Labels" },
  { id: "assignee", label: "Assignee" },
  { id: "storyPoints", label: "Story points" },
  { id: "parentKey", label: "Parent key" },
  { id: "sprint", label: "Sprint" },
];

// readFileAsBase64 reads a browser File into the base64 the import
// bindings take (the data URL's payload, after the comma).
export function readFileAsBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(reader.error ?? new Error("The file could not be read."));
    reader.onload = () => {
      const url = String(reader.result ?? "");
      resolve(url.slice(url.indexOf(",") + 1));
    };
    reader.readAsDataURL(file);
  });
}

export const PreviewImport: (contentB64: string, isXlsx: boolean) => Promise<ImportPreview> = App.PreviewImport;
export const AutoMapImport: (headers: string[]) => Promise<ImportMapping> = App.AutoMapImport;
export const ImportIssues = (
  profileId: string,
  contentB64: string,
  isXlsx: boolean,
  fileName: string,
  mapping: ImportMapping,
  dryRun: boolean,
): Promise<ImportResult> =>
  App.ImportIssues(profileId, contentB64, isXlsx, fileName, importer.Mapping.createFrom(mapping), dryRun) as Promise<ImportResult>;
export const SaveImportTemplate: (profileId: string) => Promise<string> = App.SaveImportTemplate;
