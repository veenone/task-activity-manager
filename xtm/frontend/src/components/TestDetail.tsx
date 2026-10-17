import { DetailHeader } from "./test-detail/DetailHeader";
import { FieldsSection } from "./test-detail/FieldsSection";
import { ReviewSection } from "./test-detail/ReviewSection";
import { CustomFieldsSection } from "./test-detail/CustomFieldsSection";
import { PreconditionsSection, MembershipsSection, RequirementsSection, BugsSection } from "./test-detail/LinkSections";
import { RunHistorySection } from "./test-detail/RunHistorySection";
import { DescriptionSection, CucumberEditor, GenericEditor } from "./test-detail/DefinitionSections";
import { StepsSection } from "./test-detail/StepsSection";
import { DatasetSection } from "./test-detail/DatasetSection";
import { DetailModals } from "./test-detail/DetailModals";
import type { TestDetailProps } from "./test-detail/types";
import { useTestDetailState } from "./test-detail/useTestDetailState";
import { useTestDetailLoad } from "./test-detail/useTestDetailLoad";
import { useTestDetailActions } from "./test-detail/useTestDetailActions";

// TestDetail is the slide-in panel for one test. The hooks in test-detail/
// hold what it knows, load and does; each section of the panel is its own
// component there.
export function TestDetail(p: TestDetailProps) {
  const s = useTestDetailState(p);
  useTestDetailLoad(s, p);
  const a = useTestDetailActions(s, p);
  const { width, execType, test, panelLoading } = s;

  return (
    <aside className="detail" style={{ width }}>
      <DetailHeader s={s} a={a} p={p} />

      {test && !panelLoading && (
        <div className="detail-body">
          <FieldsSection s={s} a={a} p={p} />

          <ReviewSection s={s} a={a} p={p} />

          <CustomFieldsSection s={s} p={p} />

          <PreconditionsSection s={s} a={a} p={p} />

          <MembershipsSection s={s} a={a} p={p} />

          <RequirementsSection s={s} a={a} p={p} />

          <BugsSection s={s} />

          <RunHistorySection s={s} />

          <DescriptionSection s={s} a={a} p={p} />

          {execType === "Cucumber" ? (
            <CucumberEditor s={s} a={a} p={p} />
          ) : execType === "Generic" ? (
            <GenericEditor s={s} a={a} p={p} />
          ) : (
            <StepsSection s={s} a={a} p={p} />
          )}

          <DatasetSection
            profileId={s.profileId}
            testKey={p.testKey}
            supported={s.caps.supportsDatasets}
            canEdit={s.caps.supportsDatasetWrites}
            containerKey={p.containerKey ?? ""}
          />
        </div>
      )}
      <DetailModals s={s} a={a} p={p} />
    </aside>
  );
}
