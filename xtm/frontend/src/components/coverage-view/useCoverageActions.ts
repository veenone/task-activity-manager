import { CreateCanonicalRequirement, DeleteCanonicalRequirement, SetMemberVersion, UpsertCoverageNode, DeleteCoverageNode, ImportCoverageTemplate, ExportCoverageReport, DownloadCoverageTemplate, SeedDemoCoverageExample, SeedPKCS11Reference, SeedEUICCReference, errMsg } from "../../api";
import type { CanonicalRequirement } from "../../api";
import { coverage } from "../../../wailsjs/go/models";
import type { CoverageViewProps, CoverageState } from "./useCoverageState";

// useCoverageActions is what the Coverage view does: add and delete functional
// requirements, import, export and download templates, seed the demo data, add
// and delete groups, and lock every member to the selected version.
export function useCoverageActions(s: CoverageState, p: CoverageViewProps) {
  const { profileId, confirm, selected, setSelected, versionId, setBusy, setError, setNotice, newName, setNewName, newGroupName, setNewGroupName, model, reuse, loadList, reload } = s;
  const { demoVariant, onChanged } = p;

  async function createCanonical() {
    const name = newName.trim();
    if (!name) return;
    setBusy(true);
    try {
      const id = await CreateCanonicalRequirement(profileId, name, "", "");
      setNewName("");
      await loadList();
      setSelected(id);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

  async function deleteCanonical(c: CanonicalRequirement) {
    const ok = await confirm({
      title: `Delete "${c.name}"?`,
      message: "This deletes its parameter model, mappings, and memberships, but leaves your Jira tests untouched.",
      danger: true,
    });
    if (!ok) return;
    try {
      await DeleteCanonicalRequirement(profileId, c.id);
      if (selected === c.id) setSelected("");
      await loadList();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function importTemplate() {
    if (!versionId) return;
    setBusy(true);
    setNotice("");
    try {
      const sum = await ImportCoverageTemplate(profileId, versionId);
      if (sum.groups === 0 && sum.values === 0) {
        setNotice("Import cancelled or empty.");
      } else {
        setNotice(
          `Imported ${sum.groups} groups, ${sum.values} values, ${sum.mappedTests} test mappings` +
            (sum.skipped ? ` (${sum.skipped} skipped)` : ""),
        );
      }
      await reload();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

  async function exportReport() {
    if (!versionId) return;
    setBusy(true);
    try {
      const path = await ExportCoverageReport(profileId, versionId);
      setNotice(path ? `Exported to ${path}` : "Export cancelled.");
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

  async function downloadTemplate() {
    setBusy(true);
    setNotice("");
    try {
      const path = await DownloadCoverageTemplate();
      setNotice(path ? `Template saved to ${path}` : "Download cancelled.");
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

  async function loadDemoExample() {
    setBusy(true);
    setNotice("");
    setError("");
    try {
      const id = await SeedDemoCoverageExample(profileId);
      await loadList();
      setSelected(id);
      onChanged?.();
      setNotice("Loaded the Login demo example, mapped to the demo's Login tests. Its coverage is in the header.");
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

  async function loadPkcsReference() {
    setBusy(true);
    setNotice("");
    setError("");
    try {
      const isEuicc = demoVariant === "euicc";
      const s = isEuicc
        ? await SeedEUICCReference(profileId)
        : await SeedPKCS11Reference(profileId);
      await loadList();
      onChanged?.();
      setNotice(
        isEuicc
          ? `Mapped eUICC coverage onto the synced demo-euicc data: ${s.features} features, ${s.versions} versions, ` +
              `${s.changeRequests} change requests, ${s.mappings} value-to-test mappings. ` +
              `Sync the demo-euicc profile first if you haven't already.`
          : `Mapped PKCS#11 coverage onto the synced demo-pkcs data: ${s.features} features, ${s.versions} versions, ` +
              `${s.changeRequests} change requests, ${s.mappings} value-to-test mappings. ` +
              `Sync the demo-pkcs profile first if you haven't already.`,
      );
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

  async function addGroup() {
    const name = newGroupName.trim();
    if (!name || !selected || !versionId) return;
    try {
      await UpsertCoverageNode(
        profileId,
        coverage.NodeEdit.createFrom({
          kind: "group",
          canonicalId: selected,
          versionId,
          name,
          sortOrder: model?.groups.length ?? 0,
        }),
      );
      setNewGroupName("");
      await reload();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function deleteNode(kind: string, id: string) {
    try {
      await DeleteCoverageNode(profileId, kind, id);
      await reload();
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function applyVersionToAll() {
    if (!versionId || reuse.length === 0) return;
    setBusy(true);
    try {
      await Promise.all(
        reuse.map((row) => SetMemberVersion(profileId, selected, row.requirementKey, versionId)),
      );
      await reload();
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setBusy(false);
    }
  }

  return { createCanonical, deleteCanonical, importTemplate, exportReport, downloadTemplate, loadDemoExample, loadPkcsReference, addGroup, deleteNode, applyVersionToAll };
}

export type CoverageActions = ReturnType<typeof useCoverageActions>;
