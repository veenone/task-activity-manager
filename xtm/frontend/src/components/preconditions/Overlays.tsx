import { BulkAssociatePreconditions } from "../../api";
import { AddTestsModal } from "../AddTestsModal";
import { TestDetail } from "../TestDetail";
import { ImportPreconditionsModal } from "../ImportPreconditionsModal";
import type { PreconditionsViewProps, PreconditionsState } from "./usePreconditionsState";
import { CreatePreconditionModal } from "./CreatePreconditionModal";

// Overlays are the docked test detail and the create, import and add-tests
// dialogs.
export function Overlays({ s, p }: { s: PreconditionsState; p: PreconditionsViewProps }) {
  const { profileId, selected, setSelected, showCreate, setShowCreate, showImport, setShowImport, showAdd, setShowAdd, detailKey, setDetailKey, detailVersion, setDetailVersion, tests, selectedPre } = s;
  const { onChanged, jiraUrl } = p;
  return (
    <>
      {detailKey && (
        <TestDetail
          testKey={detailKey}
          jiraUrl={jiraUrl}
          version={detailVersion}
          pendingForTest={[]}
          folders={[]}
          onClose={() => setDetailKey("")}
          onEdited={() => {
            setDetailVersion((v) => v + 1);
            onChanged();
          }}
        />
      )}

      {showCreate && (
        <CreatePreconditionModal
          profileId={profileId}
          onCancel={() => setShowCreate(false)}
          onCreated={(key) => {
            setShowCreate(false);
            setSelected(key);
            onChanged();
          }}
        />
      )}

      {showImport && (
        <ImportPreconditionsModal
          onCancel={() => setShowImport(false)}
          onComplete={() => {
            setShowImport(false);
            onChanged();
          }}
        />
      )}

      {showAdd && selectedPre && (
        <AddTestsModal
          containerKey={selected}
          targetLabel={selectedPre.key}
          existingKeys={tests.map((t) => t.key)}
          onAdd={(keys) =>
            BulkAssociatePreconditions(profileId, keys, [selected], true).then(
              () => undefined,
            )
          }
          onCancel={() => setShowAdd(false)}
          onDone={() => {
            setShowAdd(false);
            onChanged();
          }}
        />
      )}
    </>
  );
}
