import { TestTable } from "../components/TestTable";
import { TestDetail } from "../components/TestDetail";
import { NewTestPanel } from "../components/NewTestPanel";
import { FolderTree } from "../components/FolderTree";
import { ContainerList } from "../components/ContainerList";
import { ComponentList } from "../components/ComponentList";
import { Dashboard } from "../components/Dashboard";
import { TraceabilityTabs } from "../components/TraceabilityTabs";
import { ContainersView } from "../components/ContainersView";
import { DatasetsView } from "../components/datasets/DatasetsView";
import { ComponentsView } from "../components/components-admin/ComponentsView";
import { PreconditionsView } from "../components/PreconditionsView";
import { RequirementsView } from "../components/RequirementsView";
import { CoverageView } from "../components/CoverageView";
import { DuplicatesView } from "../components/DuplicatesView";
import { GapAnalysisView } from "../components/GapAnalysisView";
import { TestCallsView } from "../components/TestCallsView";
import MisspellingsView from "../components/MisspellingsView";
import type { AppState } from "./useAppState";
import type { AppActions } from "./useAppActions";

// MainView renders the chosen view. Browse, the fallback, is the grouped
// sidebar, the test table, and the detail or New Test panel beside it.
export function MainView({ s, a }: { s: AppState; a: AppActions }) {
  const { activeProfile, pulling, view, setView, groupBy, setGroupBy, selectedFolder, setSelectedFolder, selectedContainer, setSelectedContainer, selectedComponent, setSelectedComponent, showNewTest, setShowNewTest, newTestFolder, openContainer, selectedKey, setSelectedKey, detailVersion, sidebarWidth, syncState, folders, groupContainers, components, pendingByTestKey, reloadPending, refreshProfileData, isDemo, demoVar, caps, startSidebarResize } = s;
  const { createFolder, renameFolder, deleteFolder, syncTests, handleEdited, openNewTest, handleTestCreated } = a;
  return (
    <>
      {view === "preconditions" && caps.supportsPreconditionObjects ? (
        <main className="content content-preconditions">
          <PreconditionsView
            jiraUrl={activeProfile?.jiraUrl ?? ""}
            onChanged={() => {
              refreshProfileData();
              reloadPending();
            }}
          />
        </main>
      ) : view === "requirements" && caps.supportsRequirementObjects ? (
        <main className="content content-requirements">
          <RequirementsView
            onChanged={() => {
              refreshProfileData();
              reloadPending();
            }}
          />
        </main>
      ) : view === "duplicates" ? (
        <main className="content content-dashboard">
          <DuplicatesView
            folders={folders}
            pendingByTestKey={pendingByTestKey}
            onChanged={() => {
              refreshProfileData();
              reloadPending();
            }}
          />
        </main>
      ) : view === "gapanalysis" ? (
        <main className="content content-gapanalysis">
          <GapAnalysisView
            onChanged={() => {
              refreshProfileData();
              reloadPending();
            }}
          />
        </main>
      ) : view === "testcalls" ? (
        <main className="content content-dashboard">
          <TestCallsView
            onChanged={() => {
              refreshProfileData();
              reloadPending();
            }}
          />
        </main>
      ) : view === "dashboard" ? (
        <main className="content content-dashboard">
          <Dashboard onOpenDuplicates={() => setView("duplicates")} />
        </main>
      ) : view === "traceability" ? (
        <main className="content content-dashboard">
          <TraceabilityTabs jiraUrl={activeProfile?.jiraUrl ?? ""} />
        </main>
      ) : view === "plans" ? (
        <main className="content content-containers">
          <ContainersView
            isDemo={isDemo}
            jiraUrl={activeProfile?.jiraUrl ?? ""}
            onOpenTest={(k) => {
              setSelectedKey(k);
              setView("browse");
            }}
            onChanged={() => {
              refreshProfileData();
              reloadPending();
            }}
          />
        </main>
      ) : view === "datasets" && caps.supportsDatasets ? (
        <main className="content content-datasets">
          <DatasetsView
            jiraUrl={activeProfile?.jiraUrl ?? ""}
            onOpenTest={(key) => {
              setSelectedKey(key);
              setView("browse");
            }}
            onOpenContainer={(target) => {
              openContainer(target);
              setView("plans");
            }}
          />
        </main>
      ) : view === "components" && caps.supportsComponentAdmin ? (
        <main className="content content-components">
          <ComponentsView
            onChanged={() => {
              refreshProfileData();
              reloadPending();
            }}
          />
        </main>
      ) : view === "coverage" ? (
        <main className="content content-coverage">
          <CoverageView
            isDemo={isDemo}
            demoVariant={demoVar}
            onChanged={() => {
              refreshProfileData();
              reloadPending();
            }}
          />
        </main>
      ) : view === "misspellings" ? (
        <main className="content">
          <MisspellingsView
            onChanged={() => {
              refreshProfileData();
              reloadPending();
            }}
          />
        </main>
      ) : (
        <main className="content">
          <div className="browse-sidebar" style={{ width: sidebarWidth }}>
            <select
              className="groupby-select"
              value={groupBy}
              onChange={(e) => {
                setGroupBy(
                  e.target.value as
                    "folder" | "testset" | "testplan" | "component",
                );
                setSelectedFolder("");
                setSelectedKey(null);
              }}
            >
              {caps.supportsFolders && (
                <option value="folder">Group by: Folder</option>
              )}
              <option value="testset">Group by: Test Set</option>
              <option value="testplan">Group by: Test Plan</option>
              <option value="component">Group by: Component</option>
            </select>
            {groupBy === "component" ? (
              <ComponentList
                components={components}
                selected={selectedComponent}
                emptyLabel="No components synced."
                onSelect={(name) => {
                  setSelectedComponent(name);
                  setSelectedKey(null);
                }}
              />
            ) : groupBy === "folder" ? (
              !caps.supportsFolders ? (
                <div className="browse-sidebar-empty">
                  <p className="muted">
                    Folders are not supported by this backend.
                  </p>
                </div>
              ) : (
                // The tree is rendered even with nothing in it. Xray reports no
                // folders for a repository whose tests were never filed into
                // one, and replacing the tree with "No folders synced." left
                // those tests with no "All tests" row to reach them through.
                <>
                  <FolderTree
                    folders={folders}
                    selected={selectedFolder}
                    onSelect={(id) => {
                      setSelectedFolder(id);
                      setSelectedKey(null);
                    }}
                    onCreate={createFolder}
                    onRename={renameFolder}
                    onDelete={deleteFolder}
                    onNewTest={(folderId) => openNewTest(folderId)}
                    totalTests={syncState?.testCount}
                    // Kiwi reports categories as folders but cannot reshape
                    // them, so the create/rename/delete actions are hidden
                    // rather than offered and then failing.
                    readOnly={!caps.supportsFolderWrites}
                  />
                  {folders.length === 0 && (
                    <div className="browse-sidebar-empty">
                      <p className="muted">No folders synced.</p>
                      {caps.supportsFolderWrites && (
                        <button
                          className="link-btn"
                          onClick={() => createFolder("")}
                        >
                          ＋ New folder
                        </button>
                      )}
                    </div>
                  )}
                </>
              )
            ) : (
              <ContainerList
                containers={groupContainers}
                selected={selectedContainer}
                emptyLabel={
                  groupBy === "testset"
                    ? "No Test Sets synced."
                    : "No Test Plans synced."
                }
                onSelect={(key) => {
                  setSelectedContainer(key);
                  setSelectedKey(null);
                }}
              />
            )}
          </div>
          <div
            className="sidebar-resizer"
            role="presentation"
            onMouseDown={startSidebarResize}
            title="Drag to resize the sidebar"
          />
          <TestTable
            folderId={groupBy === "folder" ? selectedFolder : ""}
            containerKey={
              groupBy === "testset" || groupBy === "testplan"
                ? selectedContainer
                : ""
            }
            component={groupBy === "component" ? selectedComponent : ""}
            pendingByTestKey={pendingByTestKey}
            onSync={syncTests}
            syncing={pulling}
          />
          {showNewTest ? (
            <NewTestPanel
              folders={folders}
              initialFolderId={newTestFolder}
              onCreated={handleTestCreated}
              onCancel={() => setShowNewTest(false)}
            />
          ) : (
            selectedKey && (
              <TestDetail
                testKey={selectedKey}
                version={detailVersion}
                pendingForTest={pendingByTestKey.get(selectedKey) ?? []}
                folders={folders}
                jiraUrl={activeProfile?.jiraUrl ?? ""}
                onClose={() => setSelectedKey(null)}
                onEdited={handleEdited}
                onCloned={handleTestCreated}
              />
            )
          )}
        </main>
      )}
    </>
  );
}
