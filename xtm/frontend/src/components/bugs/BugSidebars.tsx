import { TestDetail } from "../TestDetail";
import { ContainerDetailPanel } from "../ContainerDetailPanel";
import type { BugsState, BugsPanelProps } from "./useBugsState";

// BugSidebars is the right sidebar: a read-only test, plan or execution.
export function BugSidebars({ s, p }: { s: BugsState; p: BugsPanelProps }) {
  const { setDetailKey, sidebarDetail, setSidebarDetail, detailVersion } = s;
  const { jiraUrl } = p;
  return (
    <>
      {sidebarDetail?.kind === "test" && (
        <TestDetail
          testKey={sidebarDetail.key}
          version={detailVersion}
          pendingForTest={[]}
          folders={[]}
          jiraUrl={jiraUrl}
          readOnly
          onClose={() => {
            setSidebarDetail(null);
            setDetailKey(null);
          }}
          onEdited={() => {}}
        />
      )}
      {(sidebarDetail?.kind === "plan" || sidebarDetail?.kind === "exec") && (
        <ContainerDetailPanel
          containerKey={sidebarDetail.key}
          kind={sidebarDetail.kind}
          jiraUrl={jiraUrl}
          onClose={() => setSidebarDetail(null)}
        />
      )}
    </>
  );
}
