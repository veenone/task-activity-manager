import { TokenPicker, validateLabel } from "./TokenPicker";
import { useLabels } from "../queries/app";

interface LabelsFieldProps {
  profileId: string;
  value: string;
  onChange: (v: string) => void;
  onSave: (v: string) => void;
  readOnly: boolean;
}

// LabelsField is the test detail's label control. It keeps the space-joined
// string TestDetail stores, so saveField and the dirty marker are unchanged.
// Every change saves with the new value straight away: a blur-only save missed
// chip removals, which leave nothing focused to blur.
export function LabelsField({ profileId, value, onChange, onSave, readOnly }: LabelsFieldProps) {
  const { data: buckets = [] } = useLabels(profileId);
  if (readOnly) return <span>{value || "—"}</span>;
  return (
    <TokenPicker
      label="Labels"
      value={value.split(/\s+/).filter(Boolean)}
      onChange={(next) => {
        const v = next.join(" ");
        onChange(v);
        onSave(v);
      }}
      suggestions={buckets.map((b) => b.label)}
      allowCreate
      validate={validateLabel}
      placeholder="Type to search or create"
    />
  );
}
