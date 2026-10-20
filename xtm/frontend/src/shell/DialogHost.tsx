import { ProfileForm } from "../components/ProfileForm";
import { Modal } from "../components/Modal";
import { ProfilesModal } from "../components/ProfilesModal";
import { ConnectionsModal } from "../components/ConnectionsModal";
import { BridgeWizard } from "../components/BridgeWizard";
import { PendingChangesModal } from "../components/PendingChangesModal";
import { BulkReviewModal } from "../components/BulkReviewModal";
import { REVIEW_ENABLED } from "../features";
import { BulkEditModal } from "../components/BulkEditModal";
import { BulkLabelsModal } from "../components/BulkLabelsModal";
import { BulkStepReplaceModal } from "../components/BulkStepReplaceModal";
import { BulkComponentsModal } from "../components/BulkComponentsModal";
import { BulkRenameModal } from "../components/BulkRenameModal";
import { BulkTransitionModal } from "../components/BulkTransitionModal";
import { BulkAllocateModal } from "../components/BulkAllocateModal";
import { BulkMoveModal } from "../components/BulkMoveModal";
import { BulkPreconditionsModal } from "../components/BulkPreconditionsModal";
import { BulkRequirementsModal } from "../components/BulkRequirementsModal";
import { DiagnosticsModal } from "../components/DiagnosticsModal";
import { SyncHistoryModal } from "../components/SyncHistoryModal";
import { ImportTestsModal } from "../components/ImportTestsModal";
import { LabelsModal } from "../components/labels-admin/LabelsModal";
import { AboutModal } from "../components/AboutModal";
import type { AppState } from "./useAppState";
import type { AppActions } from "./useAppActions";

// DialogHost renders whichever modal ModalContext has open, wiring each to
// the shell handlers that refresh the lists when it finishes.
export function DialogHost({ s, a }: { s: AppState; a: AppActions }) {
  const { profiles, activeId, defaultProfileId, setDefaultFor, isOpen, openModal, closeModal, syncStatus, view, setSelectedKey, selectedSet, startTour, editingProfile, setEditingProfile, lastCommitResult, pendingChanges, folders, reloadPending, refreshProfileData } = s;
  const { afterMutation, exportProfile, importProfile, deleteProfile, handleCreated, handleDiscard, handleDiscardAll, handleCommit, handleCommitIds, resolveConflictOverride, resolveConflictKeepRemote, resolveConflictMerge, resolveConflictRecreate, closePendingModal } = a;
  return (
    <>
      {isOpen("form") && (
        <Modal
          onClose={() => {
            closeModal();
            setEditingProfile(null);
          }}
          className="modal"
          label="Profile"
        >
          <ProfileForm
            profile={editingProfile ?? undefined}
            profiles={profiles}
            onCreated={handleCreated}
            onCancel={() => {
              closeModal();
              setEditingProfile(null);
            }}
          />
        </Modal>
      )}

      {isOpen("profiles") && (
        <ProfilesModal
          profiles={profiles}
          activeId={activeId}
          defaultProfileId={defaultProfileId}
          onClose={() => closeModal()}
          onSetDefault={setDefaultFor}
          onExport={exportProfile}
          onImport={importProfile}
          onSaved={handleCreated}
          onDelete={deleteProfile}
        />
      )}

      {isOpen("connections") && activeId && (
        <ConnectionsModal activeId={activeId} onClose={() => closeModal()} />
      )}

      {isOpen("bridge") && activeId && (
        <BridgeWizard
          activeId={activeId}
          onClose={() => closeModal()}
          onOpenConnections={() => {
            closeModal();
            openModal("connections");
          }}
        />
      )}

      {isOpen("pending") && (
        <PendingChangesModal
          changes={pendingChanges}
          onDiscard={handleDiscard}
          onDiscardAll={handleDiscardAll}
          onCommit={handleCommit}
          onCommitIds={handleCommitIds}
          onJumpTo={(key) => {
            setSelectedKey(key);
            closePendingModal();
          }}
          onResolveOverride={resolveConflictOverride}
          onResolveKeepRemote={resolveConflictKeepRemote}
          onResolveMerge={resolveConflictMerge}
          onResolveRecreate={resolveConflictRecreate}
          onClose={closePendingModal}
          committing={syncStatus === "committing"}
          lastResult={lastCommitResult}
        />
      )}

      {isOpen("bulkEdit") && (
        <BulkEditModal
          testKeys={[...selectedSet]}
          onComplete={() => afterMutation({ clearSelection: true })}
          onCancel={() => afterMutation()}
        />
      )}

      {isOpen("bulkLabels") && (
        <BulkLabelsModal
          testKeys={[...selectedSet]}
          onComplete={() => afterMutation({ clearSelection: true })}
          onCancel={() => afterMutation()}
        />
      )}

      {isOpen("bulkComponents") && (
        <BulkComponentsModal
          testKeys={[...selectedSet]}
          onComplete={() => afterMutation({ clearSelection: true })}
          onCancel={() => afterMutation()}
        />
      )}

      {isOpen("bulkStepReplace") && (
        <BulkStepReplaceModal
          testKeys={[...selectedSet]}
          onComplete={() => afterMutation({ clearSelection: true })}
          onCancel={() => afterMutation()}
        />
      )}

      {isOpen("bulkRename") && (
        <BulkRenameModal
          testKeys={[...selectedSet]}
          onComplete={() => afterMutation({ clearSelection: true })}
          onCancel={() => afterMutation()}
        />
      )}

      {isOpen("bulkTransition") && (
        <BulkTransitionModal
          testKeys={[...selectedSet]}
          onComplete={() => afterMutation({ clearSelection: true })}
          onCancel={() => afterMutation()}
        />
      )}

      {isOpen("bulkAllocate") && (
        <BulkAllocateModal
          testKeys={[...selectedSet]}
          onComplete={() => afterMutation({ clearSelection: true })}
          onCancel={() => afterMutation()}
        />
      )}

      {isOpen("bulkMove") && (
        <BulkMoveModal
          testKeys={[...selectedSet]}
          folders={folders}
          onComplete={() => afterMutation({ clearSelection: true })}
          onCancel={() => afterMutation()}
        />
      )}

      {isOpen("diagnostics") && (
        <DiagnosticsModal onClose={() => closeModal()} />
      )}

      {isOpen("about") && (
        <AboutModal
          onClose={() => closeModal()}
          onTakeTour={() => startTour(view)}
        />
      )}

      {isOpen("syncHistory") && (
        <SyncHistoryModal onClose={() => closeModal()} />
      )}

      {isOpen("labels") && (
        <LabelsModal
          onChanged={() => {
            refreshProfileData();
            reloadPending();
          }}
          onClose={() => closeModal()}
        />
      )}

      {isOpen("import") && (
        <ImportTestsModal
          onComplete={() => {
            refreshProfileData();
            reloadPending();
            closeModal();
          }}
          onCancel={() => closeModal()}
        />
      )}

      {isOpen("bulkPreconditions") && (
        <BulkPreconditionsModal
          testKeys={[...selectedSet]}
          onComplete={() => afterMutation({ clearSelection: true })}
          onCancel={() => afterMutation()}
        />
      )}

      {isOpen("bulkRequirements") && (
        <BulkRequirementsModal
          testKeys={[...selectedSet]}
          onComplete={() => afterMutation({ clearSelection: true })}
          onCancel={() => closeModal()}
        />
      )}

      {REVIEW_ENABLED && isOpen("bulkReview") && (
        <BulkReviewModal
          testKeys={[...selectedSet]}
          onComplete={() => afterMutation({ clearSelection: true })}
          onCancel={() => closeModal()}
        />
      )}
    </>
  );
}
