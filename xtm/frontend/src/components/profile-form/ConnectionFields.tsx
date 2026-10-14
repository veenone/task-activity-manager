import { normalizeJiraUrl } from "@agile-suite/core";
import type { ProfileFormState } from "./useProfileFormState";

// ConnectionFields are the name, role, backend, URL and project key, and the
// Xray-only scope, cross-project and bug-filing settings.
export function ConnectionFields({ s }: { s: ProfileFormState }) {
  const { name, setName, backend, setBackend, jiraUrl, setJiraUrl, projectKey, setProjectKey, scopeJql, setScopeJql, crossProjectSources, setCrossProjectSources, bugIssueType, setBugIssueType, bugProjectMode, setBugProjectMode, bugProjectKey, setBugProjectKey, role, setRole, setReuseFrom, isConnection, backendIsKiwi, keyError, urlError } = s;
  return (
    <>
      <label>
        {isConnection ? "Connection name" : "Profile name"}
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={isConnection ? "Kiwi target" : "QA — Project X"}
        />
      </label>
      {isConnection && (
        <label>
          Role
          <select value={role} onChange={(e) => setRole(e.target.value)}>
            <option value="target">Target (publish destination)</option>
            <option value="source">Source (pulled from)</option>
            <option value="both">Both</option>
          </select>
        </label>
      )}
      <label>
        Backend
        <select
          value={backend}
          onChange={(e) => {
            setBackend(e.target.value);
            setReuseFrom(""); // the reuse list is backend-filtered; drop a now-invalid selection
          }}
        >
          <option value="xray">Xray / Jira</option>
          <option value="kiwi">Kiwi TCMS</option>
        </select>
      </label>
      <label>
        {backendIsKiwi ? "Kiwi server URL" : "Jira base URL"}
        <input
          value={jiraUrl}
          onChange={(e) => setJiraUrl(e.target.value)}
          onBlur={() => setJiraUrl(normalizeJiraUrl(jiraUrl))}
          placeholder={backendIsKiwi ? "https://kiwi.example.com" : "https://jira.example.com"}
          spellCheck={false}
        />
        {urlError && <span className="field-error">{urlError}</span>}
      </label>
      <label>
        {backendIsKiwi ? "Product" : "Project key"}
        <input
          value={projectKey}
          onChange={(e) =>
            setProjectKey(backendIsKiwi ? e.target.value : e.target.value.toUpperCase())
          }
          placeholder={backendIsKiwi ? "e.g. MyProduct" : "QA"}
          spellCheck={false}
        />
        {keyError && <span className="field-error">{keyError}</span>}
      </label>
      {!backendIsKiwi && (
        <label>
          Scope JQL (optional)
          <input
            value={scopeJql}
            onChange={(e) => setScopeJql(e.target.value)}
            placeholder="e.g. labels = smoke (narrows which tests sync)"
          />
        </label>
      )}
      {!backendIsKiwi && !isConnection && (
        <label>
          Cross-project source projects (optional)
          <input
            value={crossProjectSources}
            onChange={(e) => setCrossProjectSources(e.target.value)}
            placeholder="e.g. PROJVAL, JKTEE"
            spellCheck={false}
          />
          <span className="field-hint">
            Project keys you can link preconditions, test calls, and cloned steps
            from. Separate multiple keys with a comma or space (e.g. "PROJVAL,
            JKTEE"). Leave blank to disable cross-project linking.
          </span>
        </label>
      )}
      {!backendIsKiwi && (
        <label>
          Bug issue type
          <input
            value={bugIssueType}
            onChange={(e) => setBugIssueType(e.target.value)}
            placeholder="Bug (the Jira issue type used when filing a defect)"
            spellCheck={false}
          />
        </label>
      )}
      {!backendIsKiwi && (
        <label>
          Bug project
          <select
            value={bugProjectMode}
            onChange={(e) => setBugProjectMode(e.target.value)}
          >
            <option value="test">Same as test (the test's project)</option>
            <option value="execution">Same as the Test Execution</option>
            <option value="dedicated">Dedicated project…</option>
          </select>
        </label>
      )}
      {!backendIsKiwi && bugProjectMode === "dedicated" && (
        <label>
          Dedicated bug project key
          <input
            value={bugProjectKey}
            onChange={(e) => setBugProjectKey(e.target.value.toUpperCase())}
            placeholder="e.g. DEFECTS (the project where bugs are filed)"
            spellCheck={false}
          />
        </label>
      )}
    </>
  );
}
