import { SetKeepOffline } from "../wailsjs/go/main/App";
import { errorText } from "./ProfileForm";

// KeepOffline turns on, for one saved query, keeping its issues' links on
// this computer when it runs, so the issue graph opens offline (#464).
export function KeepOffline({
  profileId,
  queryId,
  on,
  disabled,
  onChanged,
  onError,
}: {
  profileId: string;
  queryId: number;
  on: boolean;
  disabled: boolean;
  onChanged: () => void;
  onError: (message: string) => void;
}) {
  return (
    <label className="pie-check" title="Each run also fetches the issues one link away and keeps them for the issue graph to use offline">
      <input
        type="checkbox"
        checked={on}
        disabled={disabled}
        onChange={(e) => {
          SetKeepOffline(profileId, queryId, e.target.checked).then(onChanged, (err) => onError(errorText(err)));
        }}
      />
      Keep links for offline
    </label>
  );
}
