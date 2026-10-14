import { useCallback, useEffect, useMemo, useState } from "react";
import { useProfile } from "../../contexts/ProfileContext";
import { useCanonicalRequirements, useCoverageDetail } from "../../queries/coverage";
import { ListVersions, errMsg } from "../../api";
import type { Version } from "../../api";
import { useConfirm } from "../useConfirm";

export interface CoverageViewProps {
  isDemo?: boolean;
  demoVariant?: "pkcs" | "euicc" | "";
  onChanged?: () => void;
}

// useCoverageState holds what the Coverage view knows: the functional
// requirements, the selected one and its versions, the coverage detail for the
// selected version, the open tab and dialogs, and the add inputs. reload
// refreshes the versions and the detail after a change.
export function useCoverageState(p: CoverageViewProps) {
  const { onChanged } = p;
  const { activeId: profileId } = useProfile();
  // The canonical (functional-requirement) list comes from the query cache
  // (audit A3, Phase 3); a mutation refreshes it via invalidateProfileData.
  const canonQuery = useCanonicalRequirements(profileId);
  const canon = canonQuery.data ?? [];
  const listError = canonQuery.error ? errMsg(canonQuery.error) : "";
  const [selected, setSelected] = useState("");
  const [versions, setVersions] = useState<Version[]>([]);
  const [versionId, setVersionId] = useState("");
  // The per-selection coverage detail (model/report/gaps/reuse/stale) comes from
  // the query cache with a stable key (Phase 4c), refreshed by the same
  // invalidation as the canonical list.
  const detailQuery = useCoverageDetail(profileId, selected, versionId);
  const model = detailQuery.data?.model ?? null;
  const report = detailQuery.data?.report ?? null;
  const gaps = detailQuery.data?.gaps ?? [];
  const reuse = detailQuery.data?.reuse ?? [];
  const stale = detailQuery.data?.stale ?? [];
  const [tab, setTab] = useState<"guide" | "matrix" | "gaps" | "reuse" | "versions" | "map">("guide");
  const loading = detailQuery.isFetching;
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [newName, setNewName] = useState("");
  const [newGroupName, setNewGroupName] = useState("");
  const [mapping, setMapping] = useState<{ valueId: string; label: string } | null>(null);
  const [showMembers, setShowMembers] = useState(false);
  const { confirm } = useConfirm();

  // loadList refetches the canonical-requirements query (the ~4 manual call
  // sites after add/delete still work); the query auto-loads and
  // invalidateProfileData handles refresh, so no dedicated load effect is needed.
  const loadList = () => canonQuery.refetch();

  // Load versions whenever the selected canonical changes; default to first
  // stable version, else first version.
  const loadVersions = useCallback(async () => {
    if (!profileId || !selected) {
      setVersions([]);
      setVersionId("");
      return;
    }
    try {
      const vs = await ListVersions(profileId, selected);
      setVersions(vs ?? []);
      setVersionId((prev) =>
        prev && (vs ?? []).some((v) => v.id === prev)
          ? prev
          : ((vs ?? []).find((v) => v.status === "stable")?.id ?? vs?.[0]?.id ?? "")
      );
    } catch (e) {
      setError(errMsg(e));
    }
  }, [profileId, selected]);

  useEffect(() => {
    void loadVersions();
  }, [loadVersions]);

  // (The per-selection coverage detail now loads via useCoverageDetail above;
  // reload() below refetches it.)

  const reload = async () => {
    await loadVersions();
    await detailQuery.refetch();
    onChanged?.();
  };

  const staleByValue = useMemo(() => {
    const m: Record<string, number> = {};
    for (const s of stale) m[s.valueId] = (m[s.valueId] ?? 0) + 1;
    return m;
  }, [stale]);

  const selectedCanon = canon.find((c) => c.id === selected);
  const versionName = versions.find((v) => v.id === versionId)?.name ?? "";

  return { profileId, confirm, selected, setSelected, versions, setVersions, versionId, setVersionId, tab, setTab, busy, setBusy, error, setError, notice, setNotice, newName, setNewName, newGroupName, setNewGroupName, mapping, setMapping, showMembers, setShowMembers, canonQuery, canon, listError, detailQuery, model, report, gaps, reuse, stale, loading, loadList, loadVersions, reload, staleByValue, selectedCanon, versionName };
}

export type CoverageState = ReturnType<typeof useCoverageState>;
