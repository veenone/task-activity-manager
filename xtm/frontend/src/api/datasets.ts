// Parameterized test datasets and the dataset catalog.

import {
  SaveTestDataset as saveTestDatasetBinding,
  ImportDatasetCSV as importDatasetCSVBinding,
  ExportDatasetCSV as exportDatasetCSVBinding,
  SyncDatasetCatalog as syncDatasetCatalogBinding,
  ListDatasetCatalog as listDatasetCatalogBinding,
  GetDatasetCatalogDetail as getDatasetCatalogDetailBinding,
  GetParameterizedRun as getParameterizedRunBinding,
} from "../../wailsjs/go/main/App";
import { testrepo } from "../../wailsjs/go/models";

// SaveTestDataset stores a test's dataset locally and queues it for commit
// (#277 W1). The generated binding takes the model class, so the plain shape
// the rest of this module deals in is converted here instead of being cast
// away at the call site.
// ImportDatasetCSV reads a dataset out of a CSV and hands back the table to
// show, carrying over what the table being replaced knows about the
// parameters it still has (#277 W2). Nothing is stored until the user saves.
export async function ImportDatasetCSV(
  content: string,
  current: TestDataset,
): Promise<TestDataset> {
  return importDatasetCSVBinding(content, testrepo.Dataset.createFrom(current));
}

// ExportDatasetCSV writes the table to a file the user picks and returns the
// path, or "" if they cancelled.
export async function ExportDatasetCSV(testKey: string, dataset: TestDataset): Promise<string> {
  return exportDatasetCSVBinding(testKey, testrepo.Dataset.createFrom(dataset));
}

export async function SaveTestDataset(
  profileId: string,
  testKey: string,
  dataset: TestDataset,
): Promise<void> {
  await saveTestDatasetBinding(profileId, testKey, testrepo.Dataset.createFrom(dataset));
}

export async function SyncDatasetCatalog(profileId: string): Promise<void> {
  await syncDatasetCatalogBinding(profileId);
}

export async function ListDatasetCatalog(
  profileId: string,
  query: DatasetCatalogQuery,
): Promise<DatasetCatalogPage> {
  return listDatasetCatalogBinding(
    profileId,
    testrepo.DatasetCatalogQuery.createFrom(query),
  ) as Promise<DatasetCatalogPage>;
}

export async function GetDatasetCatalogDetail(
  profileId: string,
  testKey: string,
): Promise<DatasetCatalogDetail> {
  return getDatasetCatalogDetailBinding(
    profileId,
    testKey,
  ) as Promise<DatasetCatalogDetail>;
}

export async function GetParameterizedRun(
  profileId: string,
  execKey: string,
  testKey: string,
  forceRefresh: boolean,
): Promise<ParameterizedRun> {
  return getParameterizedRunBinding(
    profileId,
    execKey,
    testKey,
    forceRefresh,
  ) as Promise<ParameterizedRun>;
}

// DatasetParameter mirrors testrepo.DatasetParameter: one column of a
// parameterized test's dataset. A combinatorial parameter is expanded by
// Xray into extra iterations, so such a dataset runs more times than it has
// rows, by a rule Xray does not publish.
export interface DatasetParameter {
  name: string;
  combinatorial: boolean;
  // Everything below is Xray's own bookkeeping, read with the dataset and
  // handed back on save. A parameter written without it is rebuilt by the
  // server, which gives it a new id and turns a list into free text (#283).
  // The editor carries these through and changes none of them.
  id?: number;
  type?: string;
  options?: string[];
  listScope?: string;
  listId?: number;
}

// TestDataset mirrors testrepo.Dataset: the parameters a test's
// ${placeholder} references resolve against, and one row per iteration.
// Empty parameters means the test is not parameterized.
export interface TestDataset {
  parameters: DatasetParameter[];
  rows: string[][];
}

export interface IterationParameter {
  name: string;
  value: string;
}

export interface IterationStep {
  id: string;
  definitionId: string;
  rank: number;
  status: string;
  action: string;
  data: string;
  expected: string;
}

export interface RunIteration {
  id: string;
  rank: number;
  status: string;
  parameters: IterationParameter[];
  steps: IterationStep[];
}

export interface ParameterizedRun {
  id: string;
  executionKey: string;
  testKey: string;
  datasetSourceKind: "test" | "plan" | "execution";
  datasetSourceKey: string;
  status: string;
  iterations: RunIteration[];
}

export interface DatasetCatalogQuery {
  search: string;
  scope: string;
  ownership: string;
  limit: number;
  offset: number;
}

export interface DatasetCatalogState {
  activeGeneration: number;
  lastSyncedAt: string;
  ownedTestCount: number;
  externalTestCount: number;
  relationshipCount: number;
  testGroupCount: number;
  definitionCount: number;
}

export interface DatasetCatalogTestSummary {
  testKey: string;
  summary: string;
  projectKey: string;
  external: boolean;
  internalOpenable: boolean;
  defaultParameterCount: number;
  defaultRowCount: number;
  planCount: number;
  executionCount: number;
  overrideCount: number;
}

export interface DatasetCatalogDefinition {
  testKey: string;
  scopeKind: string;
  contextKey: string;
  contextSummary: string;
  parameters: DatasetParameter[];
  rows: string[][];
  metadataComplete: boolean;
}

export interface DatasetCatalogUsage {
  testKey: string;
  containerKind: string;
  containerKey: string;
  containerSummary: string;
}

export interface DatasetCatalogExecPlan {
  execKey: string;
  planKey: string;
}

export interface DatasetCatalogPage {
  state: DatasetCatalogState | null;
  tests: DatasetCatalogTestSummary[];
  total: number;
}

export interface DatasetCatalogDetail {
  test: DatasetCatalogTestSummary;
  definitions: DatasetCatalogDefinition[];
  usages: DatasetCatalogUsage[];
  execPlans: DatasetCatalogExecPlan[];
}
