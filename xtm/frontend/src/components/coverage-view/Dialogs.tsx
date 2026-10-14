import { MapTestsModal, MembersModal } from "./CoverageModals";
import type { CoverageState } from "./useCoverageState";

// Dialogs opens the map-tests and members dialogs.
export function Dialogs({ s }: { s: CoverageState }) {
  const { profileId, selected, mapping, setMapping, showMembers, setShowMembers, reuse, reload, selectedCanon } = s;
  return (
    <>
      {mapping && (
        <MapTestsModal
          profileId={profileId}
          canonicalId={selected}
          valueId={mapping.valueId}
          valueLabel={mapping.label}
          onClose={() => setMapping(null)}
          onSaved={async () => {
            setMapping(null);
            await reload();
          }}
        />
      )}
      {showMembers && selectedCanon && (
        <MembersModal
          profileId={profileId}
          canonicalId={selected}
          current={reuse.map((r) => r.requirementKey)}
          onClose={() => setShowMembers(false)}
          onSaved={async () => {
            setShowMembers(false);
            await reload();
          }}
        />
      )}
    </>
  );
}
