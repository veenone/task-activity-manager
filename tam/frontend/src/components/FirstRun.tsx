// FirstRun stands in for the views while no profile exists. Every view
// reads through the active profile, so without one each drew an empty state
// of its own that blamed the sync, and Dashboards said it was reading
// forever (#526). The copy follows XTM's onboarding screen; the layout is
// core's empty panel from Manage Profiles, and the button opens that dialog
// rather than repeating its form here.
export function FirstRun({ onCreate }: { onCreate: () => void }) {
  return (
    <section className="profiles-empty" aria-label="Get started">
      <div className="profiles-empty-mark">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor"
          strokeWidth="1.7" aria-hidden="true">
          <rect x="3" y="4" width="18" height="16" rx="2" />
          <path d="M3 9h18" />
          <circle cx="7" cy="6.5" r="0.6" fill="currentColor" />
        </svg>
      </div>
      <h3>Task Activity Manager</h3>
      <p>Connect to your Jira Data Center project to get started.</p>
      <button type="button" className="btn btn-primary btn-block" onClick={onCreate}>
        Create a profile
      </button>
    </section>
  );
}
