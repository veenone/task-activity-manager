import type { profile } from "../wailsjs/go/models";
import { isDemo } from "./AddProfileDialog";
import { ThemeToggle } from "./ThemeToggle";

// TopBar is the suite's header, laid out as TAM's and XTM's are: the brand and
// the profile switcher on the left, actions on the right (#452).
export function TopBar({
  profiles,
  profileId,
  running,
  fieldsReady,
  shownTheme,
  onProfile,
  onFields,
  onDeleteProfile,
  onAddProfile,
  onTheme,
}: {
  profiles: profile.Profile[] | null;
  profileId: string;
  running: boolean;
  fieldsReady: boolean;
  shownTheme: string;
  onProfile: (id: string) => void;
  onFields: () => void;
  onDeleteProfile: () => void;
  onAddProfile: () => void;
  onTheme: (theme: string) => void;
}) {
  const selected = profiles?.find((p) => p.id === profileId);
  return (
    <header className="topbar">
      <div className="topbar-zone topbar-left">
        <span className="brand">Project Issue Explorer</span>
        {selected && isDemo(selected.jiraUrl) && (
          <span className="demo-chip">DEMO</span>
        )}
        {selected && (
          <>
            <label className="sr-only" htmlFor="pie-profile">
              Profile
            </label>
            <select
              id="pie-profile"
              className="profile-select"
              value={profileId}
              disabled={running}
              onChange={(e) => onProfile(e.target.value)}
            >
              {profiles?.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          </>
        )}
      </div>
      <div className="topbar-zone topbar-right">
        {selected && (
          <>
            <button
              type="button"
              className="btn btn-ghost"
              disabled={running || !fieldsReady}
              onClick={onFields}
            >
              Jira fields
            </button>
            <button
              type="button"
              className="btn btn-ghost"
              disabled={running}
              onClick={onDeleteProfile}
            >
              Delete profile
            </button>
          </>
        )}
        <button type="button" className="btn btn-ghost" onClick={onAddProfile}>
          Add profile
        </button>
        <ThemeToggle shown={shownTheme} onChange={onTheme} />
      </div>
    </header>
  );
}
