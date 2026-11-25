// The Dependencies view's read (#612): every link of the profile, cached or
// pending, drawn from its outward issue to its inward one.

import * as App from "../../wailsjs/go/main/App";
import { issueexport } from "../../wailsjs/go/models";
import type { Issue } from "./issues";

// DependencyLink is one link once, however many of its ends cache it. owner
// and direction are the issue it is cached or journaled under and its
// direction from there, which a removal or a retype names.
export interface DependencyLink {
  id: string;
  type: string;
  outward: string;
  inward: string;
  owner: string;
  direction: string;
  pending: boolean;
  pendingId: number;
  removal: boolean;
}

// ExternalIssue is a link's far end that is not cached, such as an issue in
// another project, known only by what the link row says about it.
export interface ExternalIssue {
  key: string;
  summary: string;
  issueType: string;
}

export interface DependencyGraph {
  issues: Issue[];
  external: ExternalIssue[];
  links: DependencyLink[];
}

export const GetDependencyGraph = (profileId: string): Promise<DependencyGraph> =>
  App.GetDependencyGraph(profileId) as Promise<DependencyGraph>;

// ExportDependencies writes a workbook of the scope where the user says,
// with image (a PNG data URL, or "") on its Graph sheet. It answers with the
// path, or "" when the dialog was cancelled.
export const ExportDependencies = (profileId: string, scope: { focus: string; keys: string[]; types: string[] }, image: string): Promise<string> =>
  App.ExportDependencies(profileId, issueexport.DependencyScope.createFrom(scope), image) as Promise<string>;
