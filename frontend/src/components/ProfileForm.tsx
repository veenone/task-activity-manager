import { useState } from "react";
import { CreateProfile, TestConnection, errMsg } from "../api";
import type { Profile } from "../api";

interface Props {
  onCreated: (p: Profile) => void;
  onCancel?: () => void;
}

export function ProfileForm({ onCreated, onCancel }: Props) {
  const [name, setName] = useState("");
  const [jiraUrl, setJiraUrl] = useState("");
  const [projectKey, setProjectKey] = useState("");
  const [token, setToken] = useState("");

  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState("");
  const [testOk, setTestOk] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const canTest = jiraUrl.trim() !== "" && token.trim() !== "";
  const canSave =
    name.trim() !== "" &&
    jiraUrl.trim() !== "" &&
    projectKey.trim() !== "" &&
    token.trim() !== "";

  async function test() {
    setTesting(true);
    setTestResult("");
    setTestOk(false);
    try {
      const user = await TestConnection(jiraUrl.trim(), token.trim());
      setTestResult(`Connected as ${user}`);
      setTestOk(true);
    } catch (e) {
      setTestResult(errMsg(e));
    } finally {
      setTesting(false);
    }
  }

  async function save() {
    setSaving(true);
    setError("");
    try {
      const p = await CreateProfile(
        name.trim(),
        jiraUrl.trim(),
        projectKey.trim(),
        token.trim(),
      );
      onCreated(p);
    } catch (e) {
      setError(errMsg(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="profile-form">
      <h2>New profile</h2>
      <label>
        Profile name
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="QA — Project X"
        />
      </label>
      <label>
        Jira base URL
        <input
          value={jiraUrl}
          onChange={(e) => setJiraUrl(e.target.value)}
          placeholder="https://jira.example.com"
        />
      </label>
      <label>
        Project key
        <input
          value={projectKey}
          onChange={(e) => setProjectKey(e.target.value)}
          placeholder="QA"
        />
      </label>
      <label>
        Personal Access Token
        <input
          type="password"
          value={token}
          onChange={(e) => setToken(e.target.value)}
          placeholder="Jira PAT — stored in Windows Credential Manager"
        />
      </label>

      <div className="form-actions">
        <button className="btn" onClick={test} disabled={!canTest || testing}>
          {testing ? "Testing…" : "Test connection"}
        </button>
        {testResult && (
          <span className={testOk ? "ok-text" : "error-text"}>
            {testResult}
          </span>
        )}
      </div>

      {error && <div className="error-text">{error}</div>}

      <div className="form-actions form-actions-end">
        {onCancel && (
          <button className="btn" onClick={onCancel} disabled={saving}>
            Cancel
          </button>
        )}
        <button
          className="btn btn-primary"
          onClick={save}
          disabled={!canSave || saving}
        >
          {saving ? "Saving…" : "Create profile"}
        </button>
      </div>
    </div>
  );
}
