import { deniedSummary } from "../lib/permissions";
import { usePermissions } from "../queries/permissions";

// PermissionNote says once, in text, which permissions the token lacks. The
// controls those permissions gate are disabled with the reason in their
// title, which only a pointer can read; this is the same reason for
// everyone else, and it tells a user on a read-only token why so much is
// greyed out. It renders nothing while every permission is held or unknown.
export function PermissionNote({ profileId }: { profileId: string }) {
  const { data } = usePermissions(profileId);
  const denied = deniedSummary(data);
  if (denied.length === 0) return null;
  return (
    <p role="note" className="muted small permission-note">
      {`Your Jira account is missing permissions, so some actions here are turned off: ${denied.join(", ")}.`}
    </p>
  );
}
