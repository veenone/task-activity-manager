// The project's releases (Jira fix versions) and components, as the last
// sync cached them, and the bulk set the Backlog's selection bar sends.

import * as App from "../../wailsjs/go/main/App";

// Version is one fix version. TAM's UI calls it a release. A negative id is
// a draft made in TAM and not yet committed.
export interface Version {
  id: number;
  project: string;
  name: string;
  description: string;
  startDate: string;
  releaseDate: string;
  released: boolean;
  archived: boolean;
  draft: boolean;
}

export interface Component {
  id: number;
  project: string;
  name: string;
  description: string;
  leadName: string;
  leadDisplayName: string;
  assigneeType: string;
  draft: boolean;
}

// CatalogField is the name both fields go by in the journal and the
// bindings, which is also their JSON name on Issue.
export type CatalogField = "components" | "fixVersions";

export interface CatalogSetResult {
  changed: string[];
  skipped: string[];
}

export const GetVersions: (profileId: string) => Promise<Version[]> =
  App.GetVersions as unknown as (profileId: string) => Promise<Version[]>;

export const GetComponents: (profileId: string) => Promise<Component[]> =
  App.GetComponents as unknown as (profileId: string) => Promise<Component[]>;

// SetIssuesCatalog journals one edit per key. add keeps each issue's own
// values and adds ids to them; otherwise ids replace them.
export const SetIssuesCatalog: (profileId: string, keys: string[], field: CatalogField, ids: number[], add: boolean) => Promise<CatalogSetResult> =
  App.SetIssuesCatalog as unknown as (profileId: string, keys: string[], field: CatalogField, ids: number[], add: boolean) => Promise<CatalogSetResult>;
