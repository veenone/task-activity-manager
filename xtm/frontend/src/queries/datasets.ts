import { useQuery } from "@tanstack/react-query";
import {
  GetDatasetCatalogDetail,
  GetTestDataset,
  ListDatasetCatalog,
  type DatasetCatalogQuery,
} from "../api";
import { call } from "../lib/apiCall";
import { keys } from "./keys";

export function useDatasetCatalog(
  profileId: string,
  query: DatasetCatalogQuery,
) {
  return useQuery({
    queryKey: keys.datasetCatalog(profileId, query),
    queryFn: () => call(() => ListDatasetCatalog(profileId, query)),
    enabled: !!profileId,
    placeholderData: (previous) => previous,
  });
}

export function useDatasetCatalogDetail(
  profileId: string,
  testKey: string,
  enabled: boolean,
) {
  return useQuery({
    queryKey: keys.datasetDetail(profileId, testKey),
    queryFn: () => call(() => GetDatasetCatalogDetail(profileId, testKey)),
    enabled: !!profileId && !!testKey && enabled,
  });
}

export function useTestDataset(
  profileId: string,
  testKey: string,
  enabled: boolean,
) {
  return useQuery({
    queryKey: keys.testDataset(profileId, testKey),
    queryFn: () => call(() => GetTestDataset(profileId, testKey, false)),
    enabled: !!profileId && !!testKey && enabled,
  });
}
